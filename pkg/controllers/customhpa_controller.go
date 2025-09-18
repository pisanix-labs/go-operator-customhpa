package controllers

import (
    "context"
    "time"

    "github.com/go-logr/logr"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/reconcile"
    record "k8s.io/client-go/tools/record"

    monitoringv1alpha1 "github.com/pisanix-labs/go-operator-customhpa/pkg/api/v1alpha1"
    "github.com/pisanix-labs/go-operator-customhpa/pkg/prom"
)

const (
    finalizerName = "customhpa.pisanix.dev/finalizer"
    managedAnno   = "customhpa.pisanix.dev/managed"
)

type CustomHPAReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
    Log      logr.Logger
}

// Reconcile contains a simple custom HPA logic:
// - Evaluates PromQL; if value > 0, scales up by 1 up to max.
// - If value == 0, scales down by 1 down to min.
// - Adds finalizer and an annotation on the managed Deployment; removes them on deletion.
func (r *CustomHPAReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    _ = log.FromContext(ctx)

    var chpa monitoringv1alpha1.CustomHPA
    if err := r.Get(ctx, req.NamespacedName, &chpa); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Default namespace for target
    targetNS := chpa.Spec.TargetRef.Namespace
    if targetNS == "" {
        targetNS = chpa.Namespace
    }

    // Handle deletion & finalizer
    if chpa.DeletionTimestamp != nil {
        if controllerutil.ContainsFinalizer(&chpa, finalizerName) {
            // cleanup: remove annotation from target Deployment if exists
            var dep appsv1.Deployment
            if err := r.Get(ctx, types.NamespacedName{Name: chpa.Spec.TargetRef.Name, Namespace: targetNS}, &dep); err == nil {
                if dep.Annotations != nil && dep.Annotations[managedAnno] == chpa.Name {
                    delete(dep.Annotations, managedAnno)
                    if err := r.Update(ctx, &dep); err != nil {
                        return ctrl.Result{}, err
                    }
                }
            }
            controllerutil.RemoveFinalizer(&chpa, finalizerName)
            if err := r.Update(ctx, &chpa); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }

    // Ensure finalizer
    if !controllerutil.ContainsFinalizer(&chpa, finalizerName) {
        controllerutil.AddFinalizer(&chpa, finalizerName)
        if err := r.Update(ctx, &chpa); err != nil {
            return ctrl.Result{}, err
        }
        // Requeue quickly to continue
        return ctrl.Result{Requeue: true}, nil
    }

    // Validate min/max
    if chpa.Spec.MinReplicas < 0 || chpa.Spec.MaxReplicas <= 0 || chpa.Spec.MinReplicas > chpa.Spec.MaxReplicas {
        msg := "invalid min/max replicas"
        r.setCondition(ctx, &chpa, metav1.ConditionFalse, "InvalidSpec", msg)
        r.Recorder.Event(&chpa, corev1.EventTypeWarning, "InvalidSpec", msg)
        return ctrl.Result{}, nil
    }

    // Fetch target Deployment
    var dep appsv1.Deployment
    if err := r.Get(ctx, types.NamespacedName{Name: chpa.Spec.TargetRef.Name, Namespace: targetNS}, &dep); err != nil {
        r.setCondition(ctx, &chpa, metav1.ConditionFalse, "TargetNotFound", err.Error())
        r.Recorder.Eventf(&chpa, corev1.EventTypeWarning, "TargetNotFound", "Deployment %s/%s not found", targetNS, chpa.Spec.TargetRef.Name)
        return r.requeueAfter(chpa), client.IgnoreNotFound(err)
    }

    // Mark managed annotation
    if dep.Annotations == nil {
        dep.Annotations = map[string]string{}
    }
    if dep.Annotations[managedAnno] != chpa.Name {
        dep.Annotations[managedAnno] = chpa.Name
        if err := r.Update(ctx, &dep); err != nil {
            return ctrl.Result{}, err
        }
    }

    // Evaluate PromQL
    value, err := prom.QueryInstant(ctx, chpa.Spec.PrometheusURL, chpa.Spec.PromQL)
    if err != nil {
        r.setCondition(ctx, &chpa, metav1.ConditionFalse, "QueryFailed", err.Error())
        r.Recorder.Eventf(&chpa, corev1.EventTypeWarning, "QueryFailed", "PromQL error: %v", err)
        return r.requeueAfter(chpa), nil
    }

    desired := currentReplicas(&dep)
    if value > 0 {
        desired = desired + 1
        if desired > chpa.Spec.MaxReplicas {
            desired = chpa.Spec.MaxReplicas
        }
    } else {
        // value <= 0: scale down one
        if desired > chpa.Spec.MinReplicas {
            desired = desired - 1
        }
        if desired < chpa.Spec.MinReplicas {
            desired = chpa.Spec.MinReplicas
        }
    }

    // Apply scaling if needed
    if desired != currentReplicas(&dep) {
        dep.Spec.Replicas = &desired
        if err := r.Update(ctx, &dep); err != nil {
            r.setCondition(ctx, &chpa, metav1.ConditionFalse, "ScaleFailed", err.Error())
            r.Recorder.Eventf(&chpa, corev1.EventTypeWarning, "ScaleFailed", "Failed updating replicas to %d: %v", desired, err)
            return r.requeueAfter(chpa), err
        }
        now := metav1.Now()
        chpa.Status.LastScaleTime = &now
        r.Recorder.Eventf(&chpa, corev1.EventTypeNormal, "Scaled", "Set replicas to %d (query=%.3f)", desired, value)
    }

    // Update status
    chpa.Status.CurrentReplicas = desired
    chpa.Status.LastQueryValue = value
    r.setCondition(ctx, &chpa, metav1.ConditionTrue, "Reconciled", "Reconciliation successful")
    if err := r.Status().Update(ctx, &chpa); err != nil {
        return r.requeueAfter(chpa), err
    }

    return r.requeueAfter(chpa), nil
}

func currentReplicas(dep *appsv1.Deployment) int32 {
    if dep.Spec.Replicas == nil {
        return 1
    }
    return *dep.Spec.Replicas
}

func (r *CustomHPAReconciler) setCondition(ctx context.Context, chpa *monitoringv1alpha1.CustomHPA, status metav1.ConditionStatus, reason, msg string) {
    cond := metav1.Condition{
        Type:               "Ready",
        Status:             status,
        Reason:             reason,
        Message:            msg,
        LastTransitionTime: metav1.Now(),
        ObservedGeneration: chpa.GetGeneration(),
    }
    // replace or append
    found := false
    for i, c := range chpa.Status.Conditions {
        if c.Type == cond.Type {
            chpa.Status.Conditions[i] = cond
            found = true
            break
        }
    }
    if !found {
        chpa.Status.Conditions = append(chpa.Status.Conditions, cond)
    }
}

func (r *CustomHPAReconciler) requeueAfter(chpa monitoringv1alpha1.CustomHPA) reconcile.Result {
    interval := 30
    if chpa.Spec.IntervalSeconds != nil && *chpa.Spec.IntervalSeconds > 0 {
        interval = int(*chpa.Spec.IntervalSeconds)
    }
    return reconcile.Result{RequeueAfter: time.Duration(interval) * time.Second}
}

// SetupWithManager wires the controller watches.
func (r *CustomHPAReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&monitoringv1alpha1.CustomHPA{}).
        Owns(&appsv1.Deployment{}).
        Complete(r)
}
