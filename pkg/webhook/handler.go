package webhook

import (
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

	decisionsAllow  atomic.Uint64
	decisionsDeny   atomic.Uint64
	decisionsMutate atomic.Uint64
)

func init() {
	expvar.Publish("token_lifetime_webhook_decisions_allow", expvar.Func(func() any {
		return decisionsAllow.Load()
	}))
	expvar.Publish("token_lifetime_webhook_decisions_deny", expvar.Func(func() any {
		return decisionsDeny.Load()
	}))
	expvar.Publish("token_lifetime_webhook_decisions_mutate", expvar.Func(func() any {
		return decisionsMutate.Load()
	}))
}

// Handler serves admission reviews and health endpoints.
type Handler struct {
	Store *policy.Store
	Ready func() bool
}

// NewMux returns an HTTP mux with admission, health, and metrics routes.
func (h *Handler) NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", h.ServeMutate)
	mux.HandleFunc("/healthz", h.ServeHealthz)
	mux.HandleFunc("/readyz", h.ServeReadyz)
	mux.Handle("/metrics", expvar.Handler())
	return mux
}

// ServeHealthz always returns 200.
func (h *Handler) ServeHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ServeReadyz returns 200 when the policy informer has synced.
func (h *Handler) ServeReadyz(w http.ResponseWriter, _ *http.Request) {
	if h.Ready != nil && !h.Ready() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ServeMutate handles AdmissionReview requests for TokenRequest CREATE.
func (h *Handler) ServeMutate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		klog.V(4).Infof("admission handled in %s", time.Since(start))
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Errorf("failed to read body: %v", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	review := admissionv1.AdmissionReview{}
	if _, _, err := universalDeserializer.Decode(body, nil, &review); err != nil {
		klog.Errorf("failed to decode AdmissionReview: %v", err)
		http.Error(w, "failed to decode AdmissionReview", http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		http.Error(w, "missing AdmissionReview.request", http.StatusBadRequest)
		return
	}

	response := h.admit(review.Request)
	response.UID = review.Request.UID

	out := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: response,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		klog.Errorf("failed to encode response: %v", err)
	}
}

func (h *Handler) admit(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if req.Resource.Resource != "serviceaccounts" || req.SubResource != "token" || req.Operation != admissionv1.Create {
		decisionsAllow.Add(1)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	var tokenReq authenticationv1.TokenRequest
	if err := json.Unmarshal(req.Object.Raw, &tokenReq); err != nil {
		decisionsDeny.Add(1)
		return denied(fmt.Sprintf("failed to decode TokenRequest: %v", err))
	}

	var requested int64
	if tokenReq.Spec.ExpirationSeconds != nil {
		requested = *tokenReq.Spec.ExpirationSeconds
	}

	pol := h.Store.Get()
	decision, err := policy.Evaluate(req.Namespace, requested, pol)
	if err != nil {
		klog.Errorf("policy evaluation error ns=%s sa=%s: %v", req.Namespace, req.Name, err)
		decisionsDeny.Add(1)
		return denied(err.Error())
	}

	klog.Infof("decision=%s ns=%s sa=%s requested=%d max=%d exempt=%v reason=%q",
		decision.Action, req.Namespace, req.Name, requested, decision.MaxExpirationSecs, decision.Exempt, decision.Reason)

	switch decision.Action {
	case policy.ActionAllow:
		decisionsAllow.Add(1)
		return &admissionv1.AdmissionResponse{Allowed: true}
	case policy.ActionDeny:
		decisionsDeny.Add(1)
		return denied(decision.Reason)
	case policy.ActionMutate:
		patch, err := expirationPatch(decision.PatchExpirationSecs)
		if err != nil {
			decisionsDeny.Add(1)
			return denied(fmt.Sprintf("failed to build patch: %v", err))
		}
		pt := admissionv1.PatchTypeJSONPatch
		decisionsMutate.Add(1)
		return &admissionv1.AdmissionResponse{
			Allowed:   true,
			Patch:     patch,
			PatchType: &pt,
			Result: &metav1.Status{
				Message: decision.Reason,
			},
		}
	default:
		decisionsDeny.Add(1)
		return denied(fmt.Sprintf("unknown decision action %q", decision.Action))
	}
}

func denied(msg string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}

type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int64  `json:"value"`
}

func expirationPatch(seconds int64) ([]byte, error) {
	ops := []jsonPatchOp{{
		Op:    "add",
		Path:  "/spec/expirationSeconds",
		Value: seconds,
	}}
	return json.Marshal(ops)
}
