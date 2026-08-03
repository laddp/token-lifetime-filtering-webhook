// Code generated manually to satisfy runtime.Object. DO NOT EDIT.
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out.
func (in *TokenLifetimePolicy) DeepCopyInto(out *TokenLifetimePolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy creates a new TokenLifetimePolicy.
func (in *TokenLifetimePolicy) DeepCopy() *TokenLifetimePolicy {
	if in == nil {
		return nil
	}
	out := new(TokenLifetimePolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a new runtime.Object.
func (in *TokenLifetimePolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *TokenLifetimePolicyList) DeepCopyInto(out *TokenLifetimePolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TokenLifetimePolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a new TokenLifetimePolicyList.
func (in *TokenLifetimePolicyList) DeepCopy() *TokenLifetimePolicyList {
	if in == nil {
		return nil
	}
	out := new(TokenLifetimePolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a new runtime.Object.
func (in *TokenLifetimePolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *TokenLifetimePolicySpec) DeepCopyInto(out *TokenLifetimePolicySpec) {
	*out = *in
	if in.ExemptNamespaces != nil {
		in, out := &in.ExemptNamespaces, &out.ExemptNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy creates a new TokenLifetimePolicySpec.
func (in *TokenLifetimePolicySpec) DeepCopy() *TokenLifetimePolicySpec {
	if in == nil {
		return nil
	}
	out := new(TokenLifetimePolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *TokenLifetimePolicyStatus) DeepCopyInto(out *TokenLifetimePolicyStatus) {
	*out = *in
}

// DeepCopy creates a new TokenLifetimePolicyStatus.
func (in *TokenLifetimePolicyStatus) DeepCopy() *TokenLifetimePolicyStatus {
	if in == nil {
		return nil
	}
	out := new(TokenLifetimePolicyStatus)
	in.DeepCopyInto(out)
	return out
}
