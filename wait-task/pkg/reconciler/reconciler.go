/*
Copyright 2021 The Tekton Authors

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

package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	kreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct{}

// ReconcileKind implements Interface.ReconcileKind.
func (c *Reconciler) ReconcileKind(ctx context.Context, r *v1alpha1.Run) kreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling %s/%s", r.Namespace, r.Name)

	// Ignore completed waits.
	if r.IsDone() {
		logger.Info("Run is finished, done reconciling")
		return nil
	}

	if r.Spec.Ref == nil ||
		r.Spec.Ref.APIVersion != "example.dev/v0" || r.Spec.Ref.Kind != "Wait" {
		// This is not a Run we should have been notified about; do nothing.
		return nil
	}
	if r.Spec.Ref.Name != "" {
		r.Status.MarkRunFailed("UnexpectedName", "Found unexpected ref name: %s", r.Spec.Ref.Name)
		return nil
	}

	expr := r.Spec.GetParam("duration")
	if expr == nil || expr.Value.StringVal == "" {
		r.Status.MarkRunFailed("MissingDuration", "The duration param was not passed")
		return nil
	}
	dur, err := time.ParseDuration(expr.Value.StringVal)
	if err != nil {
		r.Status.MarkRunFailed("InvalidDuration", "The duration param was invalid: %v", err)
		return nil
	}

	// feat: 增加httpEndpoint和pipelineId参数
	var httpEndpointStr string
	httpEndpoint := r.Spec.GetParam("httpEndpoint")
	if httpEndpoint != nil && httpEndpoint.Value.StringVal != "" {
		httpEndpointStr = httpEndpoint.Value.StringVal
	}
	var pipelineIdStr string
	pipelineId := r.Spec.GetParam("pipelineId")
	if pipelineId != nil && pipelineId.Value.StringVal != "" {
		pipelineIdStr = pipelineId.Value.StringVal
	}

	logger.Infof("yaml params: duration %s httpEndpoint %s pipelineId %s", dur.String(), httpEndpointStr, pipelineIdStr)

	// 参数校验只在多的时候起作用
	if len(r.Spec.Params) > 3 {
		var found []string
		for _, p := range r.Spec.Params {
			if p.Name == "duration" || p.Name == "httpEndpoint" || p.Name == "pipelineId" {
				continue
			}
			found = append(found, p.Name)
		}
		r.Status.MarkRunFailed("UnexpectedParams", "Found unexpected params: %v", found)
		return nil
	}

	if r.Status.StartTime == nil {
		now := metav1.Now()
		r.Status.StartTime = &now
		r.Status.MarkRunRunning("Waiting", "Waiting for duration to elapse")
	}

	// 增加httpEndpoint属性，获取当前任务状态
	if httpEndpointStr != "" && pipelineIdStr != "" {
		urlStr := fmt.Sprintf("%s?pipelineId=%s", httpEndpointStr, pipelineIdStr)
		resp, err := httpClient().Get(urlStr)
		if err != nil {
			// requeue
			r.Status.MarkRunRunning("UnexpectedHttpErr", fmt.Sprintf("Failed to check http endpoint %s", urlStr))
			return controller.NewRequeueAfter(dur)
		}

		// 解析resp，判断是否可以继续当前的pipeline
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// requeue
			r.Status.MarkRunRunning("UnexpectedHttpStatusCode", fmt.Sprintf("Failed to check http endpoint %s status code %d", urlStr, resp.StatusCode))
			return controller.NewRequeueAfter(dur)
		}

		b, _ := ioutil.ReadAll(resp.Body)
		logger.Infof("http response, url: %s body: %s", urlStr, string(b))

		hr := httpResponse{}
		if err := json.Unmarshal(b, &hr); err != nil {
			// requeue
			r.Status.MarkRunRunning("UnexpectedResponse", fmt.Sprintf("url %s response %s", urlStr, string(b)))
			return controller.NewRequeueAfter(dur)
		}
		if hr.Status == Success || hr.Status == Fail {
			now := metav1.Now()
			r.Status.CompletionTime = &now
			r.Status.MarkRunSucceeded(hr.Status, hr.Reason)

			// feat: 支持结果传递，wait-task
			r.Status.Results = append(r.Status.Results, hr.Kvs...)
			return nil
		}

		// requeue
		r.Status.MarkRunRunning("Checking", fmt.Sprintf("Got response %s %s", urlStr, string(b)))
		return controller.NewRequeueAfter(dur)
	} else {
		done := r.Status.StartTime.Time.Add(dur)

		if time.Now().After(done) {
			now := metav1.Now()
			r.Status.CompletionTime = &now
			r.Status.MarkRunSucceeded("DurationElapsed", "The wait duration has elapsed")
		} else {
			// Enqueue another check when the timeout should be elapsed.
			return controller.NewRequeueAfter(dur)
		}
	}

	// Don't emit events on nop-reconciliations, it causes scale problems.
	return nil
}

const (
	Success = "SUCCESS"
	Fail    = "FAIL"
)

type httpResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`

	// pre-publish/publish场景：
	// []*v1alpha1.RunResult{
	// 	{"name": "JENKINS_JOB_NAME", "value": ""},
	// 	{"name": "JENKINS_BUILD_NO", "value": ""},
	// 	{"name": "JENKINS_BUILD_URL", "value": ""},
	// 	{"name": "JENKINS_BUILD_TYPE", "value": ""},
	// }
	Kvs []v1alpha1.RunResult `json:"kvs"`
}

func httpClient() *http.Client {
	httpDialContextFunc := (&net.Dialer{Timeout: 1 * time.Second, DualStack: true}).DialContext
	return &http.Client{
		Transport: &http.Transport{
			DialContext: httpDialContextFunc,

			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 0,

			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 50,
		},
		Timeout: 3 * time.Second,
	}
}
