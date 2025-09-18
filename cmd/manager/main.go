package main

import (
    "flag"
    "os"
    "time"
    "strconv"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/cache"
    "sigs.k8s.io/controller-runtime/pkg/healthz"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
    metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

    monitoringv1alpha1 "github.com/pisanix-labs/go-operator-customhpa/pkg/api/v1alpha1"
    "github.com/pisanix-labs/go-operator-customhpa/pkg/controllers"
)

var (
    scheme   = runtime.NewScheme()
    setupLog = ctrl.Log.WithName("setup")
)

func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    utilruntime.Must(appsv1.AddToScheme(scheme))
    utilruntime.Must(corev1.AddToScheme(scheme))
    utilruntime.Must(monitoringv1alpha1.AddToScheme(scheme))
}

func main() {
    var metricsAddr string
    var probeAddr string
    var enableLeaderElection bool

    flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
    flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
    flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
    opts := zap.Options{Development: true}
    opts.BindFlags(flag.CommandLine)
    flag.Parse()

    ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme: scheme,
        Metrics: metricsserver.Options{BindAddress: metricsAddr},
        HealthProbeBindAddress: probeAddr,
        LeaderElection:   enableLeaderElection,
        LeaderElectionID: "customhpa.pisanix.dev",
        Cache: cache.Options{SyncPeriod: ptrDuration(30 * time.Second)},
    })
    if err != nil {
        setupLog.Error(err, "unable to start manager")
        os.Exit(1)
    }

    // Read desired replicas from env var CHPA_DESIRED_REPLICAS (default 1)
    var desiredReplicas int32 = 1
    if v := os.Getenv("CHPA_DESIRED_REPLICAS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n >= 0 {
            desiredReplicas = int32(n)
        } else {
            setupLog.Info("invalid CHPA_DESIRED_REPLICAS, using default 1", "value", v)
        }
    }

    if err = (&controllers.CustomHPAReconciler{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        Recorder: mgr.GetEventRecorderFor("customhpa-controller"),
        Log:      ctrl.Log.WithName("controllers").WithName("CustomHPA"),
        DesiredReplicas: desiredReplicas,
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "CustomHPA")
        os.Exit(1)
    }

    if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
        setupLog.Error(err, "unable to set up health check")
        os.Exit(1)
    }
    if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
        setupLog.Error(err, "unable to set up ready check")
        os.Exit(1)
    }

    setupLog.Info("starting manager")
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        setupLog.Error(err, "problem running manager")
        os.Exit(1)
    }
}

func ptrDuration(d time.Duration) *time.Duration { return &d }
