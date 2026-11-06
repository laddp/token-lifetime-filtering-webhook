package webhook_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	tokenv1alpha1 "github.com/pladd/token-lifetime-filtering-webhook/api/v1alpha1"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/webhook"
)

func int64ptr(v int64) *int64 { return &v }

func reviewFor(ns, sa string, expiration *int64) admissionv1.AdmissionReview {
	tr := authenticationv1.TokenRequest{
		TypeMeta: metav1.TypeMeta{APIVersion: "authentication.k8s.io/v1", Kind: "TokenRequest"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      sa,
		},
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: expiration,
		},
	}
	raw, _ := json.Marshal(tr)
	return admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Request: &admissionv1.AdmissionRequest{
			UID:         "test-uid",
			Kind:        metav1.GroupVersionKind{Group: "authentication.k8s.io", Version: "v1", Kind: "TokenRequest"},
			Resource:    metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
			SubResource: "token",
			Name:        sa,
			Namespace:   ns,
			Operation:   admissionv1.Create,
			Object:      runtime.RawExtension{Raw: raw},
		},
	}
}

func post(t *testing.T, h *webhook.Handler, review admissionv1.AdmissionReview) *admissionv1.AdmissionReview {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.NewMux().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var out admissionv1.AdmissionReview
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func TestHandlerDenyMutateAllow(t *testing.T) {
	store := policy.NewStore()
	store.Upsert(&tokenv1alpha1.TokenLifetimePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: tokenv1alpha1.TokenLifetimePolicySpec{
			MaxExpiration:    "1h",
			OnExceed:         tokenv1alpha1.OnExceedDeny,
			ExemptNamespaces: []string{"openshift-*"},
		},
	})
	h := &webhook.Handler{Store: store, Ready: func() bool { return true }}

	t.Run("allow under limit", func(t *testing.T) {
		out := post(t, h, reviewFor("default", "builder", int64ptr(1800)))
		if !out.Response.Allowed {
			t.Fatalf("expected allow: %v", out.Response.Result)
		}
	})

	t.Run("deny over limit", func(t *testing.T) {
		out := post(t, h, reviewFor("default", "builder", int64ptr(7200)))
		if out.Response.Allowed {
			t.Fatal("expected deny")
		}
	})

	t.Run("exempt namespace", func(t *testing.T) {
		out := post(t, h, reviewFor("openshift-monitoring", "prometheus", int64ptr(7200)))
		if !out.Response.Allowed {
			t.Fatalf("expected allow: %v", out.Response.Result)
		}
	})

	store.Upsert(&tokenv1alpha1.TokenLifetimePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: tokenv1alpha1.TokenLifetimePolicySpec{
			MaxExpiration: "1h",
			OnExceed:      tokenv1alpha1.OnExceedMutate,
		},
	})

	t.Run("mutate over limit", func(t *testing.T) {
		out := post(t, h, reviewFor("default", "builder", int64ptr(7200)))
		if !out.Response.Allowed {
			t.Fatalf("expected allow with patch: %v", out.Response.Result)
		}
		if out.Response.Patch == nil || out.Response.PatchType == nil {
			t.Fatal("expected JSON patch")
		}
		var ops []map[string]any
		if err := json.Unmarshal(out.Response.Patch, &ops); err != nil {
			t.Fatal(err)
		}
		if len(ops) != 1 || ops[0]["path"] != "/spec/expirationSeconds" {
			t.Fatalf("unexpected patch: %v", ops)
		}
		if int64(ops[0]["value"].(float64)) != 3600 {
			t.Fatalf("unexpected value: %v", ops[0]["value"])
		}
	})
}

func TestReadyz(t *testing.T) {
	h := &webhook.Handler{
		Store: policy.NewStore(),
		Ready: func() bool { return false },
	}
	rr := httptest.NewRecorder()
	h.NewMux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestMalformedTokenPolicyScope(t *testing.T) {
	for _, tt := range []struct {
		name       string
		configured bool
		namespace  string
		wantAllow  bool
	}{
		{"no policy", false, "default", true},
		{"exempt namespace", true, "openshift-monitoring", true},
		{"enforced namespace", true, "default", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := policy.NewStore()
			if tt.configured {
				store.Upsert(&tokenv1alpha1.TokenLifetimePolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec:       tokenv1alpha1.TokenLifetimePolicySpec{MaxExpiration: "1h", ExemptNamespaces: []string{"openshift-*"}},
				})
			}
			review := reviewFor(tt.namespace, "builder", nil)
			review.Request.Object.Raw = []byte(`{"spec":{"expirationSeconds":"invalid"}}`)
			out := post(t, &webhook.Handler{Store: store}, review)
			if out.Response.Allowed != tt.wantAllow || len(out.Response.Patch) != 0 {
				t.Fatalf("unexpected response: %+v", out.Response)
			}
			if !tt.wantAllow && (out.Response.Result == nil || !strings.Contains(out.Response.Result.Message, "failed to decode TokenRequest")) {
				t.Fatalf("expected decode error: %+v", out.Response)
			}
		})
	}
}

func TestHandlerExpirationBoundaries(t *testing.T) {
	for _, mode := range []tokenv1alpha1.OnExceed{"", tokenv1alpha1.OnExceedDeny, tokenv1alpha1.OnExceedMutate} {
		for _, tt := range []struct {
			name       string
			expiration *int64
			enforce    bool
		}{
			{"under limit", int64ptr(3599), false},
			{"at limit", int64ptr(3600), false},
			{"over limit", int64ptr(3601), true},
			{"unset", nil, true},
			{"zero", int64ptr(0), true},
			{"negative", int64ptr(-1), true},
		} {
			t.Run(string(mode)+"/"+tt.name, func(t *testing.T) {
				store := policy.NewStore()
				store.Upsert(&tokenv1alpha1.TokenLifetimePolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec:       tokenv1alpha1.TokenLifetimePolicySpec{MaxExpiration: "1h", OnExceed: mode},
				})
				out := post(t, &webhook.Handler{Store: store}, reviewFor("default", "builder", tt.expiration))
				wantAllow := !tt.enforce || mode != tokenv1alpha1.OnExceedDeny
				wantPatch := tt.enforce && mode != tokenv1alpha1.OnExceedDeny
				if out.Response.Allowed != wantAllow || (len(out.Response.Patch) > 0) != wantPatch {
					t.Fatalf("unexpected response: %+v", out.Response)
				}
				if wantPatch {
					var ops []struct {
						Path  string `json:"path"`
						Value int64  `json:"value"`
					}
					if err := json.Unmarshal(out.Response.Patch, &ops); err != nil {
						t.Fatal(err)
					}
					if len(ops) != 1 || ops[0].Path != "/spec/expirationSeconds" || ops[0].Value != 3600 {
						t.Fatalf("unexpected patch: %s", out.Response.Patch)
					}
				}
			})
		}
	}
}
