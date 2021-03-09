/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	context "context"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	applyconfigurationsbatchv1 "k8s.io/client-go/applyconfigurations/batch/v1"
	gentype "k8s.io/client-go/gentype"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

// JobsGetter has a method to return a JobInterface.
// A group's client should implement this interface.
type JobsGetter interface {
	Jobs(namespace string) JobInterface
}

// zhou: "/apis/batch/v1/jobs"

// JobInterface has methods to work with Job resources.
type JobInterface interface {
	Create(ctx context.Context, job *batchv1.Job, opts metav1.CreateOptions) (*batchv1.Job, error)
	Update(ctx context.Context, job *batchv1.Job, opts metav1.UpdateOptions) (*batchv1.Job, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, job *batchv1.Job, opts metav1.UpdateOptions) (*batchv1.Job, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*batchv1.Job, error)
	List(ctx context.Context, opts metav1.ListOptions) (*batchv1.JobList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *batchv1.Job, err error)
	Apply(ctx context.Context, job *applyconfigurationsbatchv1.JobApplyConfiguration, opts metav1.ApplyOptions) (result *batchv1.Job, err error)
	// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
	ApplyStatus(ctx context.Context, job *applyconfigurationsbatchv1.JobApplyConfiguration, opts metav1.ApplyOptions) (result *batchv1.Job, err error)
	JobExpansion
}

// jobs implements JobInterface
type jobs struct {
	*gentype.ClientWithListAndApply[*batchv1.Job, *batchv1.JobList, *applyconfigurationsbatchv1.JobApplyConfiguration]
}

// newJobs returns a Jobs
func newJobs(c *BatchV1Client, namespace string) *jobs {
	return &jobs{
		gentype.NewClientWithListAndApply[*batchv1.Job, *batchv1.JobList, *applyconfigurationsbatchv1.JobApplyConfiguration](
			"jobs",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *batchv1.Job { return &batchv1.Job{} },
			func() *batchv1.JobList { return &batchv1.JobList{} },
			gentype.PrefersProtobuf[*batchv1.Job](),
		),
	}
}
