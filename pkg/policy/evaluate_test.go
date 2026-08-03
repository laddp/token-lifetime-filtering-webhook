package policy_test

import (
	"testing"

	tokenv1alpha1 "github.com/pladd/token-lifetime-filtering-webhook/api/v1alpha1"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsExempt(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		patterns  []string
		want      bool
		wantErr   bool
	}{
		{name: "empty patterns", namespace: "default", patterns: nil, want: false},
		{name: "exact match", namespace: "kube-system", patterns: []string{"kube-system"}, want: true},
		{name: "openshift star", namespace: "openshift-monitoring", patterns: []string{"openshift", "openshift-*", "kube-*"}, want: true},
		{name: "bare openshift", namespace: "openshift", patterns: []string{"openshift", "openshift-*", "kube-*"}, want: true},
		{name: "kube star", namespace: "kube-public", patterns: []string{"openshift", "openshift-*", "kube-*"}, want: true},
		{name: "user ns not exempt", namespace: "my-app", patterns: []string{"openshift", "openshift-*", "kube-*"}, want: false},
		{name: "default not exempt", namespace: "default", patterns: []string{"openshift", "openshift-*", "kube-*"}, want: false},
		{name: "invalid pattern", namespace: "default", patterns: []string{"[unterminated"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := policy.IsExempt(tt.namespace, tt.patterns)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	denyPolicy := &tokenv1alpha1.TokenLifetimePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: tokenv1alpha1.TokenLifetimePolicySpec{
			MaxExpiration:    "1h",
			OnExceed:         tokenv1alpha1.OnExceedDeny,
			ExemptNamespaces: []string{"openshift-*"},
		},
	}
	mutatePolicy := denyPolicy.DeepCopy()
	mutatePolicy.Spec.OnExceed = tokenv1alpha1.OnExceedMutate

	t.Run("nil policy allows", func(t *testing.T) {
		d, err := policy.Evaluate("default", 999999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionAllow {
			t.Fatalf("got %s", d.Action)
		}
	})

	t.Run("exempt namespace allows", func(t *testing.T) {
		d, err := policy.Evaluate("openshift-monitoring", 999999, denyPolicy)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionAllow || !d.Exempt {
			t.Fatalf("got %+v", d)
		}
	})

	t.Run("within limit allows", func(t *testing.T) {
		d, err := policy.Evaluate("default", 1800, denyPolicy)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionAllow {
			t.Fatalf("got %s", d.Action)
		}
	})

	t.Run("over limit deny", func(t *testing.T) {
		d, err := policy.Evaluate("default", 7200, denyPolicy)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionDeny {
			t.Fatalf("got %s", d.Action)
		}
	})

	t.Run("over limit mutate", func(t *testing.T) {
		d, err := policy.Evaluate("default", 7200, mutatePolicy)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionMutate || d.PatchExpirationSecs != 3600 {
			t.Fatalf("got %+v", d)
		}
	})

	t.Run("unset expiration enforced", func(t *testing.T) {
		d, err := policy.Evaluate("default", 0, denyPolicy)
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != policy.ActionDeny {
			t.Fatalf("got %s", d.Action)
		}
	})

	t.Run("invalid maxExpiration", func(t *testing.T) {
		bad := denyPolicy.DeepCopy()
		bad.Spec.MaxExpiration = "nope"
		_, err := policy.Evaluate("default", 1, bad)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestStoreSingleton(t *testing.T) {
	store := policy.NewStore()
	if store.Get() != nil {
		t.Fatal("expected nil")
	}

	ignored := &tokenv1alpha1.TokenLifetimePolicy{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
	store.Upsert(ignored)
	if store.Get() != nil {
		t.Fatal("non-cluster policy should be ignored")
	}

	cluster := &tokenv1alpha1.TokenLifetimePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: tokenv1alpha1.SingletonName},
		Spec:       tokenv1alpha1.TokenLifetimePolicySpec{MaxExpiration: "1h"},
	}
	store.Upsert(cluster)
	got := store.Get()
	if got == nil || got.Spec.MaxExpiration != "1h" {
		t.Fatalf("unexpected %+v", got)
	}

	store.Delete(tokenv1alpha1.SingletonName)
	if store.Get() != nil {
		t.Fatal("expected nil after delete")
	}
}
