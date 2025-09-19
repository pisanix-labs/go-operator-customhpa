# CustomHPA Operator (Golang)

Um operator simples em Go que implementa um "Custom HPA" (Horizontal Pod Autoscaler) baseado em configuração do próprio operator. A cada intervalo, o controller lê uma quantidade desejada de réplicas a partir de uma env var (`CHPA_DESIRED_REPLICAS`) e ajusta a escala de um Deployment alvo, respeitando limites mínimos e máximos.

Este repositório é intencionalmente simples e modular para estudo dos principais conceitos de Operators no Kubernetes.

## Visão Geral

- Grupo/Versão/Recurso: `monitoring.pisanix.dev/v1alpha1`, `CustomHPA` (plural `customhpas`).
- Alvo suportado: `Deployment` (apps/v1).
- Lógica: lê `CHPA_DESIRED_REPLICAS` e aplica esse valor, limitado por `minReplicas`/`maxReplicas`.
- Segurança/Recursos: usa RBAC para gerenciar `CustomHPA`, `Deployments` e emitir eventos.
- Finalizers: adiciona um finalizer no `CustomHPA` e uma anotação no Deployment alvo; remove na deleção (demonstração de limpeza).
- Eventos: emite eventos para sucessos e falhas importantes (ex.: "Scaled", "QueryFailed").

## Conceitos Essenciais

- CR (Custom Resource): recurso customizado criado por você para estender a API do Kubernetes. Aqui, `CustomHPA` é um CR que descreve limites (`minReplicas`/`maxReplicas`), intervalo e o alvo (`targetRef`) a ser escalado (arquivo `pkg/api/v1alpha1/customhpa_types.go`). O valor desejado vem da configuração do operator.

- CRD (CustomResourceDefinition): define o schema e o comportamento do seu CR na API do Kubernetes. Sem o CRD, o cluster não conhece o novo tipo. O CRD deste projeto está em `config/crd/bases/monitoring.pisanix.dev_customhpas.yaml`, com validação OpenAPI, subresource `status` e colunas extras.

- Operator: é um aplicativo (rodando dentro ou fora do cluster) que implementa automação operacional para um domínio específico, observando CRs e reconciliando o estado real com o estado desejado. Neste repo, o operator está em Go usando `controller-runtime` e roda o controller `CustomHPAReconciler`.

- Controller Pattern: um controller observa recursos (via informers) e reage a eventos, enfileirando pedidos de reconciliação. Este projeto usa `controller-runtime` para registrar o controller e watchers, ver `pkg/controllers/customhpa_controller.go` e `cmd/manager/main.go`.

- Reconciliation Loop: a função `Reconcile` busca o CR, valida, observa o estado atual do alvo (Deployment e réplicas), calcula o desejado a partir da env `CHPA_DESIRED_REPLICAS` (com clamp entre `min`/`max`) e aplica as mudanças necessárias, então atualiza o `status`. A reconciliação é idempotente e requeue periódica é usada pelo `RequeueAfter` (intervalo configurável via `spec.intervalSeconds`).

- Finalizers: strings adicionadas na lista de finalizers do CR para garantir limpeza antes da remoção real do objeto. Aqui, o finalizer `customhpa.pisanix.dev/finalizer` garante que a anotação `customhpa.pisanix.dev/managed` seja removida do Deployment gerenciado quando o `CustomHPA` for deletado.

- Events & Informers:
  - Informers: caches/watchers gerenciados pelo `controller-runtime`, que observa `CustomHPA` e (own) `Deployments`. Embora a lógica seja simples, sob o capô o controller usa informers para eficiência e reatividade.
  - Events: o controller registra eventos do tipo `Normal` e `Warning` (por exemplo, `Scaled`, `QueryFailed`, `TargetNotFound`) para facilitar o troubleshooting com `kubectl describe`.

## Estrutura do Projeto

- `cmd/manager/main.go`: ponto de entrada do manager (configura logs, healthz, registra o controller e inicia o manager).
- `pkg/api/v1alpha1/customhpa_types.go`: tipos do CRD (Spec/Status) e registro no scheme.
- `pkg/controllers/customhpa_controller.go`: reconciliation loop, finalizer, emissão de eventos e aplicação de escala no Deployment alvo.
- (Removido) Integração com Prometheus/PromQL.
- `config/crd/bases/*.yaml`: manifesto do CRD.
- `config/rbac/*.yaml`: RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding).
- `config/manager/deployment.yaml`: Deployment do controller (imagem exemplo, ajuste conforme seu registry).
- `config/samples/*.yaml`: exemplo de `CustomHPA` para testar.

## Exemplo de CR (resumo)

Arquivo `config/samples/monitoring_v1alpha1_customhpa.yaml`:

```yaml
apiVersion: monitoring.pisanix.dev/v1alpha1
kind: CustomHPA
metadata:
  name: sample-web-chpa
  namespace: default
spec:
  minReplicas: 1
  maxReplicas: 5
  intervalSeconds: 30
  targetRef:
    name: sample-web # Deployment apps/v1
    namespace: default
```

Semântica simplificada:
- `minReplicas`/`maxReplicas`: limites de escala.
- `intervalSeconds`: período de reconciliação (polling).
- `targetRef`: identifica o Deployment alvo (neste exemplo, `apps/v1`, nome `sample-web`).

## Configuração do Operator (réplicas desejadas)

- Defina a env `CHPA_DESIRED_REPLICAS` no Deployment do controller para indicar o número desejado de réplicas.
- O controller aplica `desired = clamp(CHPA_DESIRED_REPLICAS, minReplicas, maxReplicas)` para cada CR reconciliado.
- Exemplo no manifesto do operator (`config/manager/deployment.yaml`):

```
env:
  - name: CHPA_DESIRED_REPLICAS
    value: "2"
```

## Ambiente local

- O script ops/kind/script.sh inicializa o clusters no kind e gera o kubeconfig, é necessário ter o kind instalado, após isso basta rodar o script
- Na raiz do projeto execute o seguinte comando:

```
./ops/kind/script.sh
```

## Implantação

1) Aplique o CRD e o RBAC:

```
kubectl apply -f config/crd/bases/monitoring.pisanix.dev_customhpas.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
kubectl apply -f config/rbac/service_account.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
kubectl apply -f config/rbac/cluster_role.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
kubectl apply -f config/rbac/cluster_role_binding.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

2) Ajuste a imagem e a env `CHPA_DESIRED_REPLICAS` no `config/manager/deployment.yaml` e aplique o controller:

```
kubectl apply -f config/manager/deployment.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

3) Suba o Deployment alvo `sample-web` (NGINX) e depois crie o CR de exemplo:

```
# Deployment simples do NGINX
kubectl apply -f ops/k8s/sample-web-deployment.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml

# (Opcional) Aguarde o rollout concluir
kubectl rollout status deploy/sample-web --kubeconfig=./ops/kind/kubeconfig-kind.yaml

# Em seguida, aplique o CR CustomHPA que gerenciará o sample-web
kubectl apply -f config/samples/monitoring_v1alpha1_customhpa.yaml --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

Valide rapidamente o Deployment:

```
kubectl get deploy,pods -l app=sample-web --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

4) Observe o comportamento:

- `kubectl get customhpa -A --kubeconfig=./ops/kind/kubeconfig-kind.yaml` para listar.
- `kubectl describe customhpa sample-web-chpa --kubeconfig=./ops/kind/kubeconfig-kind.yaml` para ver condições e eventos (ex.: `Scaled`).
- `kubectl get deploy sample-web -o jsonpath='{.spec.replicas}' --kubeconfig=./ops/kind/kubeconfig-kind.yaml` para inspecionar réplicas.

## Alterar réplicas via variável de ambiente (ao vivo)

Você pode ajustar o número de réplicas desejadas alterando a env `CHPA_DESIRED_REPLICAS` no Deployment do operator e observar o efeito no Deployment alvo (respeitando `minReplicas`/`maxReplicas`).

Passo a passo (usando o cluster Kind deste repo):

1) Verifique o valor atual da env no operator:

```
kubectl set env deploy/customhpa-controller --list -n default --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

2) Altere o valor desejado (ex.: de 2 para 4):

```
kubectl set env deploy/customhpa-controller -n default \
  CHPA_DESIRED_REPLICAS=4 --kubeconfig=./ops/kind/kubeconfig-kind.yaml

kubectl rollout status deploy/customhpa-controller -n default --kubeconfig=./ops/kind/kubeconfig-kind.yaml
```

3) Observe a reconciliação e a escala do alvo:

```
# Eventos e condições do CustomHPA
kubectl describe customhpa sample-web-chpa --kubeconfig=./ops/kind/kubeconfig-kind.yaml

# Réplicas aplicadas ao Deployment alvo
kubectl get deploy sample-web -o jsonpath='{.spec.replicas}' --kubeconfig=./ops/kind/kubeconfig-kind.yaml; echo
```

4) (Opcional) Teste o limite por `maxReplicas`: defina um valor acima do máximo (ex.: 10) e observe que o operator fará clamp para `maxReplicas` (5 no exemplo):

```
kubectl set env deploy/customhpa-controller -n default \
  CHPA_DESIRED_REPLICAS=10 --kubeconfig=./ops/kind/kubeconfig-kind.yaml
kubectl rollout status deploy/customhpa-controller -n default --kubeconfig=./ops/kind/kubeconfig-kind.yaml
kubectl get deploy sample-web -o jsonpath='{.spec.replicas}' --kubeconfig=./ops/kind/kubeconfig-kind.yaml; echo
```

Notas:
- A reconciliação ocorre periodicamente conforme `spec.intervalSeconds` do CR (ex.: 30s). Após o rollout do operator com a nova env, aguarde até um ciclo para a aplicação no alvo.
- Rodando o controller localmente com `go run ./cmd/manager`, exporte a env no seu shell antes de iniciar: `export CHPA_DESIRED_REPLICAS=4 && go run ./cmd/manager` (reinicie o processo para alterar o valor).

## Build e Deploy com Makefile

- Ajuste `IMG` conforme seu registry (por padrão `ghcr.io/pisanix-labs/customhpa-controller:latest`).

Comandos úteis:

```
make docker-build IMG=ghcr.io/pisanix-labs/customhpa-controller:latest
make docker-push IMG=ghcr.io/pisanix-labs/customhpa-controller:latest
make deploy IMG=ghcr.io/pisanix-labs/customhpa-controller:latest
```

Para rodar local (fora do cluster) com seu `KUBECONFIG` atual (defina `CHPA_DESIRED_REPLICAS` antes de executar):

```
go run ./cmd/manager
```

## Notas e Limitações (para estudo)

- Foco em `Deployment` como alvo. Para generalizar (StatefulSet, CRDs com subresource `scale` etc.), seria necessário usar a `Scale` subresource e RESTMapper/Discovery para mapear recursos.
- O valor global `CHPA_DESIRED_REPLICAS` é aplicado a todos os `CustomHPA` e limitado por `min`/`max` de cada CR (não há lógica por métrica nesta versão).
- Sem geração de `DeepCopy` e manifests via kubebuilder; o código é didático e pode requerer ajustes/`go mod tidy` para compilar/rodar em sua máquina.

## Próximos Passos Sugeridos

- Adicionar suporte à subresource `scale` genérica (para qualquer `kind` com `scale`).
- Introduzir thresholds e histerese (evitar flapping), além de janelas separadas de `scaleUp`/`scaleDown`.
- Expor métricas do controller (ex.: tempo de reconciliação, erros) e dashboards.
- Adicionar testes unitários de reconciliação.
