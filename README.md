# Token Lifetime Filtering Webhook

Mutating admission webhook for OpenShift/Kubernetes that enforces a maximum lifetime on ServiceAccount `TokenRequest` objects (`oc create token` / `serviceaccounts/token`).

## Behavior

A singleton cluster-scoped CR drives enforcement:

```yaml
apiVersion: tokenfiltering.openshift.io/v1alpha1
kind: TokenLifetimePolicy
metadata:
  name: cluster   # singleton name — required
spec:
  maxExpiration: 1y          # human-readable (e.g. 24h, 1h30m, 7d); default 1y
  onExceed: Mutate           # Deny | Mutate (default Mutate)
  exemptNamespaces:          # globs; matching namespaces skip enforcement
    - openshift
    - openshift-*
    - kube-*
```

| Situation | Result |
|-----------|--------|
| No `TokenLifetimePolicy/cluster` | Allow unchanged |
| Namespace matches `exemptNamespaces` | Allow unchanged |
| Positive `expirationSeconds` ≤ max | Allow |
| Over limit + `onExceed: Deny` | Reject |
| Over limit + `onExceed: Mutate` | Patch `spec.expirationSeconds` to the max |
| Unset/nonpositive expiration | Treated as unbounded → enforce (deny or mutate) |

Over-limit requests are mutated by default. Set `onExceed: Deny` to reject them instead.

### Duration units

`maxExpiration` accepts `s`, `m`, `h`, `d`, `w`, `y` (case-insensitive), including composites like `1h30m` or `2d12h`. Empty defaults to `1y` (365 days). Durations exceeding the maximum signed 64-bit integer seconds are rejected, including overflow in composite durations.

## Install (OpenShift)

1. Build and push the image referenced by the Kustomize deployment (the default is `quay.io/example/token-lifetime-webhook:latest`), then deploy it. The deployment uses `imagePullPolicy: Always`, so reapplying Kustomize picks up a newly pushed `latest` image without editing manifests.
2. Apply the CRD and workload, wait for readiness, then install the sample policy and register the webhook:

```bash
make deploy
# or:
oc apply -f config/crd
oc wait --for=condition=Established crd/tokenlifetimepolicies.tokenfiltering.openshift.io --timeout=120s
oc apply -k config/deploy
oc rollout status deployment/token-lifetime-webhook -n token-lifetime-webhook --timeout=180s
oc apply -f config/samples/tokenlifetimepolicy-cluster.yaml
oc apply -f config/webhook
oc wait --for=jsonpath='{.webhooks[0].clientConfig.caBundle}' mutatingwebhookconfiguration/token-lifetime-webhook --timeout=120s
```

TLS is provided by the OpenShift service CA:

- Service annotation `service.beta.openshift.io/serving-cert-secret-name`
- MutatingWebhookConfiguration annotation `service.beta.openshift.io/inject-cabundle`

Register the webhook only after its pods are Ready: its own service-account token mounts must succeed before a fail-closed webhook can serve requests. Keep the workload (`config/deploy`) and registration (`config/webhook`) steps separate. Wait for CA injection before verifying enforcement; token requests can temporarily fail during that injection window.

The webhook registration excludes the `token-lifetime-webhook` namespace so the webhook does not intercept its own ServiceAccount token requests during startup or recovery. The deployment also uses pod anti-affinity and a PodDisruptionBudget to preserve one available replica during voluntary disruptions.

`make undeploy` removes the webhook registration first. Policy-side namespace exemptions cannot bypass an unavailable webhook. If all webhook replicas are unavailable and token mounts prevent recovery, an administrator can remove registration with `oc delete -f config/webhook`, restore the workload, and repeat the readiness and registration steps above. Token requests are unenforced while registration is removed.

## Verify

```bash
# Should succeed (within default 1y) or be denied/mutated depending on policy:
oc create token default -n default --duration=2h

# With onExceed: Deny and maxExpiration: 1h, this fails:
oc create token default -n default --duration=2h

# Platform namespaces are exempt in the sample policy:
oc create token prometheus-k8s -n openshift-monitoring --duration=720h
```

Apply an alternative maximum of `30d12h` in mutate mode with:

```bash
oc apply -f config/samples/tokenlifetimepolicy-mutate.yaml
```

## Development

```bash
make test
make build
```

The build produces `bin/token-lifetime-webhook`. The container runs the same binary as `/token-lifetime-webhook`.

Flags:

- `--listen` (default `:8443`)
- `--tls-cert` / `--tls-key`
- `--kubeconfig` (optional; in-cluster config by default)

Endpoints: `/mutate`, `/healthz`, `/readyz`, `/metrics` (stdlib `expvar` JSON counters).

## Limitations

- Does **not** replace kube-apiserver `--service-account-max-token-expiration`.
- Does **not** cover OAuth access tokens (`OAuthAccessToken`); those resources are webhook-restricted on OpenShift.
- Only the singleton named `cluster` is honored.
