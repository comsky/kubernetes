/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	"flag"
	"fmt"

	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	imageutils "k8s.io/kubernetes/test/utils/image"
	psaapi "k8s.io/pod-security-admission/api"
	psapolicy "k8s.io/pod-security-admission/policy"
	"k8s.io/utils/pointer"
)

// NodeOSDistroIs returns true if the distro is the same as `--node-os-distro`
// the package framework/pod can't import the framework package (see #81245)
// we need to check if the --node-os-distro=windows is set and the framework package
// is the one that's parsing the flags, as a workaround this method is looking for the same flag again
// TODO: replace with `framework.NodeOSDistroIs` when #81245 is complete
func NodeOSDistroIs(distro string) bool {
	var nodeOsDistro *flag.Flag = flag.Lookup("node-os-distro")
	if nodeOsDistro != nil && nodeOsDistro.Value.String() == distro {
		return true
	}
	return false
}

// GenerateScriptCmd generates the corresponding command lines to execute a command.
func GenerateScriptCmd(command string) []string {
	var commands []string
	commands = []string{"/bin/sh", "-c", command}
	return commands
}

// GetDefaultTestImage returns the default test image based on OS.
// If the node OS is windows, currently we return Agnhost image for Windows node
// due to the issue of #https://github.com/kubernetes-sigs/windows-testing/pull/35.
// If the node OS is linux, return busybox image
func GetDefaultTestImage() string {
	return imageutils.GetE2EImage(GetDefaultTestImageID())
}

// GetDefaultTestImageID returns the default test image id based on OS.
// If the node OS is windows, currently we return Agnhost image for Windows node
// due to the issue of #https://github.com/kubernetes-sigs/windows-testing/pull/35.
// If the node OS is linux, return busybox image
func GetDefaultTestImageID() imageutils.ImageID {
	return GetTestImageID(imageutils.BusyBox)
}

// GetTestImage returns the image name with the given input
// If the Node OS is windows, currently we return Agnhost image for Windows node
// due to the issue of #https://github.com/kubernetes-sigs/windows-testing/pull/35.
func GetTestImage(id imageutils.ImageID) string {
	if NodeOSDistroIs("windows") {
		return imageutils.GetE2EImage(imageutils.Agnhost)
	}
	return imageutils.GetE2EImage(id)
}

// GetTestImageID returns the image id with the given input
// If the Node OS is windows, currently we return Agnhost image for Windows node
// due to the issue of #https://github.com/kubernetes-sigs/windows-testing/pull/35.
func GetTestImageID(id imageutils.ImageID) imageutils.ImageID {
	if NodeOSDistroIs("windows") {
		return imageutils.Agnhost
	}
	return id
}

// GeneratePodSecurityContext generates the corresponding pod security context with the given inputs
// If the Node OS is windows, currently we will ignore the inputs and return nil.
// TODO: Will modify it after windows has its own security context
func GeneratePodSecurityContext(fsGroup *int64, seLinuxOptions *v1.SELinuxOptions) *v1.PodSecurityContext {
	if NodeOSDistroIs("windows") {
		return nil
	}
	return &v1.PodSecurityContext{
		FSGroup:        fsGroup,
		SELinuxOptions: seLinuxOptions,
	}
}

// GenerateContainerSecurityContext generates the corresponding container security context with the given inputs
// If the Node OS is windows, currently we will ignore the inputs and return nil.
// TODO: Will modify it after windows has its own security context
func GenerateContainerSecurityContext(privileged bool) *v1.SecurityContext {
	if NodeOSDistroIs("windows") {
		return nil
	}
	return &v1.SecurityContext{
		Privileged: &privileged,
	}
}

// GetLinuxLabel returns the default SELinuxLabel based on OS.
// If the node OS is windows, it will return nil
func GetLinuxLabel() *v1.SELinuxOptions {
	if NodeOSDistroIs("windows") {
		return nil
	}
	return &v1.SELinuxOptions{
		Level: "s0:c0,c1"}
}

// DefaultNonRootUser is the default user ID used for running restricted (non-root) containers.
const DefaultNonRootUser = 1000

// GetRestrictedPodSecurityContext returns a restricted pod security context.
// This includes setting RunAsUser for convenience, to pass the RunAsNonRoot check.
// Tests that require a specific user ID should override this.
func GetRestrictedPodSecurityContext() *v1.PodSecurityContext {
	return &v1.PodSecurityContext{
		RunAsNonRoot:   pointer.BoolPtr(true),
		RunAsUser:      pointer.Int64(DefaultNonRootUser),
		SeccompProfile: &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
	}
}

// GetRestrictedContainerSecurityContext returns a minimal restricted container security context.
func GetRestrictedContainerSecurityContext() *v1.SecurityContext {
	return &v1.SecurityContext{
		AllowPrivilegeEscalation: pointer.BoolPtr(false),
		Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
	}
}

var psaEvaluator, _ = psapolicy.NewEvaluator(psapolicy.DefaultChecks())

// MustMixinRestrictedPodSecurity makes the given pod compliant with the restricted pod security level.
// If doing so would overwrite existing non-conformant configuration, a test failure is triggered.
func MustMixinRestrictedPodSecurity(pod *v1.Pod) *v1.Pod {
	err := MixinRestrictedPodSecurity(pod)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	return pod
}

// MixinRestrictedPodSecurity makes the given pod compliant with the restricted pod security level.
// If doing so would overwrite existing non-conformant configuration, an error is returned.
// Note that this sets a default RunAsUser. See GetRestrictedPodSecurityContext.
// TODO(#105919): Handle PodOS for windows pods.
func MixinRestrictedPodSecurity(pod *v1.Pod) error {
	if pod.Spec.SecurityContext == nil {
		pod.Spec.SecurityContext = GetRestrictedPodSecurityContext()
	} else {
		if pod.Spec.SecurityContext.RunAsNonRoot == nil {
			pod.Spec.SecurityContext.RunAsNonRoot = pointer.BoolPtr(true)
		}
		if pod.Spec.SecurityContext.RunAsUser == nil {
			pod.Spec.SecurityContext.RunAsUser = pointer.Int64Ptr(DefaultNonRootUser)
		}
		if pod.Spec.SecurityContext.SeccompProfile == nil {
			pod.Spec.SecurityContext.SeccompProfile = &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault}
		}
	}
	for i := range pod.Spec.Containers {
		mixinRestrictedContainerSecurityContext(&pod.Spec.Containers[i])
	}
	for i := range pod.Spec.InitContainers {
		mixinRestrictedContainerSecurityContext(&pod.Spec.InitContainers[i])
	}

	// Validate the resulting pod against the restricted profile.
	restricted := psaapi.LevelVersion{
		Level:   psaapi.LevelRestricted,
		Version: psaapi.LatestVersion(),
	}
	if agg := psapolicy.AggregateCheckResults(psaEvaluator.EvaluatePod(restricted, &pod.ObjectMeta, &pod.Spec)); !agg.Allowed {
		return fmt.Errorf("failed to make pod %s restricted: %s", pod.Name, agg.ForbiddenDetail())
	}

	return nil
}

// mixinRestrictedContainerSecurityContext adds the required container security context options to
// be compliant with the restricted pod security level. Non-conformance checking is handled by the
// caller.
func mixinRestrictedContainerSecurityContext(container *v1.Container) {
	if container.SecurityContext == nil {
		container.SecurityContext = GetRestrictedContainerSecurityContext()
	} else {
		if container.SecurityContext.AllowPrivilegeEscalation == nil {
			container.SecurityContext.AllowPrivilegeEscalation = pointer.Bool(false)
		}
		if container.SecurityContext.Capabilities == nil {
			container.SecurityContext.Capabilities = &v1.Capabilities{}
		}
		if len(container.SecurityContext.Capabilities.Drop) == 0 {
			container.SecurityContext.Capabilities.Drop = []v1.Capability{"ALL"}
		}
	}
}
