package policy

import (
	"fmt"
	"path"
	"strings"

	tokenv1alpha1 "github.com/pladd/token-lifetime-filtering-webhook/api/v1alpha1"
	"github.com/pladd/token-lifetime-filtering-webhook/pkg/duration"
)

// Action is the enforcement action chosen by Evaluate.
type Action string

const (
	ActionAllow  Action = "allow"
	ActionDeny   Action = "deny"
	ActionMutate Action = "mutate"
)

// Decision is the result of evaluating a TokenRequest against a policy.
type Decision struct {
	Action              Action
	Reason              string
	MaxExpirationSecs   int64
	PatchExpirationSecs int64
	Exempt              bool
}

// Evaluate decides whether a TokenRequest should be allowed, denied, or mutated.
// A nil policy means no singleton is configured → allow.
// requestedSeconds <= 0 (unset/zero) is treated as an unbounded request and is enforced.
func Evaluate(namespace string, requestedSeconds int64, policy *tokenv1alpha1.TokenLifetimePolicy) (Decision, error) {
	if policy == nil {
		return Decision{
			Action: ActionAllow,
			Reason: "no TokenLifetimePolicy/cluster configured",
		}, nil
	}

	exempt, err := IsExempt(namespace, policy.Spec.ExemptNamespaces)
	if err != nil {
		return Decision{}, err
	}
	if exempt {
		return Decision{
			Action: ActionAllow,
			Reason: fmt.Sprintf("namespace %q is exempt", namespace),
			Exempt: true,
		}, nil
	}

	maxSecs, err := duration.Parse(policy.Spec.MaxExpiration)
	if err != nil {
		return Decision{}, fmt.Errorf("invalid maxExpiration %q: %w", policy.Spec.MaxExpiration, err)
	}

	onExceed := policy.Spec.OnExceed
	if onExceed == "" {
		onExceed = tokenv1alpha1.OnExceedDeny
	}

	// Unset/zero expiration is treated as unbounded and must be enforced.
	if requestedSeconds > 0 && requestedSeconds <= maxSecs {
		return Decision{
			Action:            ActionAllow,
			Reason:            fmt.Sprintf("requested %ds within max %ds", requestedSeconds, maxSecs),
			MaxExpirationSecs: maxSecs,
		}, nil
	}

	requestedDesc := "unspecified"
	if requestedSeconds > 0 {
		requestedDesc = fmt.Sprintf("%ds", requestedSeconds)
	}

	switch onExceed {
	case tokenv1alpha1.OnExceedMutate:
		return Decision{
			Action:              ActionMutate,
			Reason:              fmt.Sprintf("requested %s exceeds max %ds; mutating to max", requestedDesc, maxSecs),
			MaxExpirationSecs:   maxSecs,
			PatchExpirationSecs: maxSecs,
		}, nil
	case tokenv1alpha1.OnExceedDeny:
		return Decision{
			Action:            ActionDeny,
			Reason:            fmt.Sprintf("requested %s exceeds max %ds", requestedDesc, maxSecs),
			MaxExpirationSecs: maxSecs,
		}, nil
	default:
		return Decision{}, fmt.Errorf("invalid onExceed %q", onExceed)
	}
}

// IsExempt reports whether namespace matches any shell-style glob in patterns.
// An empty pattern list means no exemptions.
func IsExempt(namespace string, patterns []string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matched, err := path.Match(pattern, namespace)
		if err != nil {
			return false, fmt.Errorf("invalid exemptNamespaces pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
