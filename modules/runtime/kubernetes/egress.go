package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const sourcePodSelector = managedByLabel + "=" + managedByValue + "," + roleLabel + "=" + roleEnvironment

// ResolveRuntimeRef maps a network source IP back to exactly one Hacocoon-owned
// Kubernetes Environment Pod. The Pod label is not sufficient authority by
// itself: the containing namespace is re-read and must carry the exact managed
// ownership labels before the provider-local runtime ref is returned.
//
// Real CNI acceptance must still prove that connections reaching the trusted
// egress boundary preserve a source identity that can be mapped this way. A
// cluster that SNATs Environment traffic before that boundary fails this parity
// path unless another equally strong provider-trusted identity mechanism is used.
func (p *Provider) ResolveRuntimeRef(ctx context.Context, source net.IP) (string, error) {
	if p == nil || p.runner == nil || source == nil || source.IsUnspecified() || source.IsLoopback() || source.IsMulticast() {
		return "", core.ErrPolicyDenied
	}
	canonical := source.String()
	if net.ParseIP(canonical) == nil {
		return "", core.ErrPolicyDenied
	}
	result, err := p.runner.Run(ctx, p.kubectl, "get", "pods", "-A", "-l", sourcePodSelector, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("list Kubernetes Environment Pods for source resolution: %w", err)
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name      string            `json:"name"`
				Namespace string            `json:"namespace"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				PodIP string `json:"podIP"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &list); err != nil {
		return "", fmt.Errorf("decode Kubernetes Environment Pod source state: %w", core.ErrIncompatibleState)
	}

	matches := make([]string, 0, 1)
	for _, pod := range list.Items {
		if strings.TrimSpace(pod.Status.PodIP) != canonical {
			continue
		}
		name := strings.TrimSpace(pod.Metadata.Labels[environmentLabel])
		ref := strings.TrimSpace(pod.Metadata.Namespace)
		if pod.Metadata.Name != podName || name == "" || ref != namespaceFor(name) {
			return "", fmt.Errorf("Kubernetes source Pod identity is not canonical Hacocoon Environment state: %w", core.ErrIncompatibleState)
		}
		namespace, err := p.namespace(ctx, ref)
		if err != nil {
			return "", err
		}
		if namespace == nil {
			return "", fmt.Errorf("Kubernetes source namespace %q disappeared during identity resolution: %w", ref, core.ErrIncompatibleState)
		}
		if err := validateOwnedNamespace(namespace, ref, name); err != nil {
			return "", err
		}
		matches = append(matches, ref)
	}
	if len(matches) != 1 {
		return "", core.ErrPolicyDenied
	}
	return matches[0], nil
}
