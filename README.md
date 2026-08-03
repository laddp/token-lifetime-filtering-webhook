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
  maxExpiration: 1y          # human-readable; default 1y
  onExceed: Deny             # Deny | Mutate (default Deny)
  exemptNamespaces:          # globs; matching namespaces skip enforcement
    - openshift
    - openshift-*
    - kube-*
```

| Situation | Result |
|-----------|--------|
| No `TokenLifetimePolicy/cluster` | Allow unchanged |
| Namespace matches `exemptNamespaces` | Allow unchanged |
| `expirationSeconds` ≤ max | Allow |
| Over limit + `onExceed: Deny` | Reject |
| Over limit + `onExceed: Mutate` | Patch `spec.expirationSeconds` to the max |
| Unset/`0` expiration | Treated as unbounded → enforce (deny or mutate) |

### Duration units

`maxExpiration` accepts `s`, `m`, `h`, `d`, `w`, `y` (case-insensitive), including compounds like `1h30m`. Empty defaults to `1y` (365 days).

## Install (OpenShift)

1. Build and push an image, then set it in `config/deploy/manifests.yaml` (or `make docker-build IMG=...`).
2. Apply CRD, deploy manifests, and sample policy:

```bash
make deploy
# or:
oc apply -f config/crd
oc apply -f config/deploy
oc apply -f config/samples/tokenlifetimepolicy-cluster.yaml
```

TLS is provided by the OpenShift service CA:

- Service annotation `service.beta.openshift.io/serving-cert-secret-name`
- MutatingWebhookConfiguration annotation `service.beta.openshift.io/inject-cabundle`

Wait until the CA bundle is injected and pods are Ready before expecting enforcement.

## Verify

```bash
# Should succeed (within default 1y) or be denied/mutated depending on policy:
oc create token default -n default --duration=2h

# With onExceed: Deny and maxExpiration: 1h, this fails:
oc create token default -n default --duration=2h

# Platform namespaces are exempt in the sample policy:
oc create token prometheus-k8s -n openshift-monitoring --duration=720h
```

Switch to mutate mode with:

```bash
oc apply -f config/samples/tokenlifetimepolicy-mutate.yaml
```

## Development

```bash
make test
make build
```

Flags:

- `--listen` (default `:8443`)
- `--tls-cert` / `--tls-key`
- `--kubeconfig` (optional; in-cluster config by default)

Endpoints: `/mutate`, `/healthz`, `/readyz`, `/metrics`.

## Limitations

- Does **not** replace kube-apiserver `--service-account-max-token-expiration`.
- Does **not** cover OAuth access tokens (`OAuthAccessToken`); those resources are webhook-restricted on OpenShift.
- Only the singleton named `cluster` is honored.
