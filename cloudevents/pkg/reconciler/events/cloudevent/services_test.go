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

package cloudevent

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/test/diff"
	"testing"

	cdeevents "github.com/cdfoundation/sig-events/cde/sdk/go/pkg/cdf/events"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

var (
	serviceDeployedEventType   = &EventType{Type: cdeevents.ServiceDeployedEventV1}
	serviceRolledbackEventType = &EventType{Type: cdeevents.ServiceRolledbackEventV1}
)

func TestServiceEventsForTaskRun(t *testing.T) {
	taskRunTests := []struct {
		desc      string
		taskRun   *v1beta1.TaskRun
		wantError bool
	}{{
		desc:      "taskrun with no annotations",
		taskRun:   getTaskRunByCondition(corev1.ConditionUnknown, v1beta1.TaskRunReasonStarted.String()),
		wantError: true,
	}, {
		desc: "taskrun with annotation, started",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.TaskRunReasonStarted.String(),
			map[string]string{ServiceDeployedEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: true,
	}, {
		desc: "taskrun with annotation, finished, failed",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionFalse,
			"meh",
			map[string]string{ServiceDeployedEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: true,
	}, {
		desc: "taskrun with annotation, finished, succeeded",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionTrue,
			"yay",
			map[string]string{ServiceDeployedEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: false,
	}}

	for _, c := range taskRunTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := getServiceDeployedEventType(c.taskRun)
			if err != nil {
				if !c.wantError {
					t.Fatalf("I did not expect an error but I got %s", err)
				}
			} else {
				if c.wantError {
					t.Fatalf("I did expect an error but I got %s", got)
				}
				if d := cmp.Diff(serviceDeployedEventType, got); d != "" {
					t.Errorf("Wrong Event Type %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestServiceEventsForPipelineRun(t *testing.T) {
	pipelineRunTests := []struct {
		desc        string
		pipelineRun *v1beta1.PipelineRun
		wantError   bool
	}{{
		desc:        "pipelinerun with no annotations",
		pipelineRun: getPipelineRunByCondition(corev1.ConditionUnknown, v1beta1.PipelineRunReasonStarted.String()),
		wantError:   true,
	}, {
		desc: "pipelinerun with annotation, started",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonStarted.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: true,
	}, {
		desc: "pipelinerun with annotation, finished, failed",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionFalse,
			"meh",
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: true,
	}, {
		desc: "pipelinerun with annotation, finished, succeeded",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionTrue,
			"yay",
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{}),
		wantError: false,
	}}

	for _, c := range pipelineRunTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := getServiceRolledbackEventType(c.pipelineRun)
			if err != nil {
				if !c.wantError {
					t.Fatalf("I did not expect an error but I got %s", err)
				}
			} else {
				if c.wantError {
					t.Fatalf("I did expect an error but I got %s", got)
				}
				if d := cmp.Diff(serviceRolledbackEventType, got); d != "" {
					t.Errorf("Wrong Event Type %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetServiceEventDataPipelineRun(t *testing.T) {
	pipelineRunTests := []struct {
		desc        string
		pipelineRun *v1beta1.PipelineRun
		wantData    CDECloudEventData
		wantError   bool
	}{{
		desc: "pipelinerun with default results, all",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonStarted.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantData: CDECloudEventData{
			"serviceEnvId":   "test123",
			"serviceVersion": "v123",
			"serviceName":    "testService"},
		wantError: false,
	}, {
		desc: "pipelinerun with default results, missing",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonStarted.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
			}),
		wantData:  nil,
		wantError: true,
	}, {
		desc: "pipelinerun with overwritten results, all",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonStarted.String(),
			map[string]string{
				ServiceRolledbackEventAnnotation.String():                 "",
				serviceMappings["serviceEnvId"].annotationResultNameKey:   "cluster",
				serviceMappings["serviceVersion"].annotationResultNameKey: "tag"},
			map[string]string{
				"cluster":         "test123",
				"cd.service.name": "testimage",
				"tag":             "v123",
			}),
		wantData: CDECloudEventData{
			"serviceEnvId":   "test123",
			"serviceVersion": "v123",
			"serviceName":    "testimage"},
		wantError: false,
	}, {
		desc: "pipelinerun with overwritten results, missing an overwritten one",
		pipelineRun: getPipelineRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonStarted.String(),
			map[string]string{
				ServiceRolledbackEventAnnotation.String():                 "",
				serviceMappings["serviceEnvId"].annotationResultNameKey:   "builtImage",
				serviceMappings["serviceVersion"].annotationResultNameKey: "tag"},
			map[string]string{
				"builtImage":      "test123",
				"cd.service.name": "testimage",
			}),
		wantData:  nil,
		wantError: true,
	}}

	for _, c := range pipelineRunTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := getServiceEventData(c.pipelineRun)
			if err != nil {
				if !c.wantError {
					t.Fatalf("I did not expect an error but I got %s", err)
				}
			} else {
				if c.wantError {
					t.Fatalf("I did expect an error but I got %s", got)
				}
				opt := cmpopts.IgnoreMapEntries(func(k string, v string) bool { return k == "pipelinerun" })
				if d := cmp.Diff(c.wantData, got, opt); d != "" {
					t.Errorf("Wrong Event Data %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetServiceEventDataTaskRun(t *testing.T) {
	taskRunTests := []struct {
		desc      string
		taskRun   *v1beta1.TaskRun
		wantData  CDECloudEventData
		wantError bool
	}{{
		desc: "taskrun with default results, all",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.TaskRunReasonStarted.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantData: CDECloudEventData{
			"serviceEnvId":   "test123",
			"serviceVersion": "v123",
			"serviceName":    "testService"},
		wantError: false,
	}, {
		desc: "taskrun with default results, missing",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.TaskRunReasonStarted.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
			}),
		wantData:  nil,
		wantError: true,
	}, {
		desc: "taskrun with overwritten results, all",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.TaskRunReasonStarted.String(),
			map[string]string{
				ServiceRolledbackEventAnnotation.String():                 "",
				serviceMappings["serviceEnvId"].annotationResultNameKey:   "builtImage",
				serviceMappings["serviceVersion"].annotationResultNameKey: "tag"},
			map[string]string{
				"builtImage":      "test123",
				"cd.service.name": "testimage",
				"tag":             "v123",
			}),
		wantData: CDECloudEventData{
			"serviceEnvId":   "test123",
			"serviceVersion": "v123",
			"serviceName":    "testimage"},
		wantError: false,
	}, {
		desc: "taskrun with overwritten results, missing an overwritten one",
		taskRun: getTaskRunByConditionAndResults(
			corev1.ConditionUnknown,
			v1beta1.TaskRunReasonStarted.String(),
			map[string]string{
				ServiceRolledbackEventAnnotation.String():                 "",
				serviceMappings["serviceEnvId"].annotationResultNameKey:   "builtImage",
				serviceMappings["serviceVersion"].annotationResultNameKey: "tag"},
			map[string]string{
				"builtImage":      "test123",
				"cd.service.name": "testimage",
			}),
		wantData:  nil,
		wantError: true,
	}}

	for _, c := range taskRunTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := getServiceEventData(c.taskRun)
			if err != nil {
				if !c.wantError {
					t.Fatalf("I did not expect an error but I got %s", err)
				}
			} else {
				if c.wantError {
					t.Fatalf("I did expect an error but I got %s", got)
				}
				opt := cmpopts.IgnoreMapEntries(func(k string, v string) bool { return k == "taskrun" })
				if d := cmp.Diff(c.wantData, got, opt); d != "" {
					t.Errorf("Wrong Event Data %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestServiceRolledbackEvent(t *testing.T) {
	serviceRolledbackTests := []struct {
		desc                string
		object              interface{}
		wantEventExtensions map[string]interface{}
	}{{
		desc: "service event for taskrun",
		object: getTaskRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.TaskRunReasonSuccessful.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}, {
		desc: "service event for pipelinerun",
		object: getPipelineRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.PipelineRunReasonSuccessful.String(),
			map[string]string{ServiceRolledbackEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}}

	for _, c := range serviceRolledbackTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := serviceRolledbackEventForObjectWithCondition(c.object.(objectWithCondition))
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				extensions := got.Extensions()
				if d := cmp.Diff(c.wantEventExtensions, extensions); d != "" {
					t.Errorf("Wrong Event Extenstions %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestServiceDeployedEvent(t *testing.T) {
	serviceDeployedTests := []struct {
		desc                string
		object              interface{}
		wantEventExtensions map[string]interface{}
	}{{
		desc: "service event for taskrun",
		object: getTaskRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.TaskRunReasonSuccessful.String(),
			map[string]string{
				ServiceDeployedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}, {
		desc: "service event for pipelinerun",
		object: getPipelineRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.PipelineRunReasonSuccessful.String(),
			map[string]string{
				ServiceDeployedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}}

	for _, c := range serviceDeployedTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := serviceDeployedEventForObjectWithCondition(c.object.(objectWithCondition))
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				extensions := got.Extensions()
				if d := cmp.Diff(c.wantEventExtensions, extensions); d != "" {
					t.Errorf("Wrong Event Extenstions %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestServiceUpgradedEvent(t *testing.T) {
	serviceDeployedTests := []struct {
		desc                string
		object              interface{}
		wantEventExtensions map[string]interface{}
	}{{
		desc: "service event for taskrun",
		object: getTaskRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.TaskRunReasonSuccessful.String(),
			map[string]string{
				ServiceUpgradedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}, {
		desc: "service event for pipelinerun",
		object: getPipelineRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.PipelineRunReasonSuccessful.String(),
			map[string]string{
				ServiceUpgradedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}}

	for _, c := range serviceDeployedTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := serviceUpgradedEventForObjectWithCondition(c.object.(objectWithCondition))
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				extensions := got.Extensions()
				if d := cmp.Diff(c.wantEventExtensions, extensions); d != "" {
					t.Errorf("Wrong Event Extenstions %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestServiceRemovedEvent(t *testing.T) {
	serviceDeployedTests := []struct {
		desc                string
		object              interface{}
		wantEventExtensions map[string]interface{}
	}{{
		desc: "service event for taskrun",
		object: getTaskRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.TaskRunReasonSuccessful.String(),
			map[string]string{
				ServiceRemovedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}, {
		desc: "service event for pipelinerun",
		object: getPipelineRunByConditionAndResults(
			corev1.ConditionTrue,
			v1beta1.PipelineRunReasonSuccessful.String(),
			map[string]string{
				ServiceRemovedEventAnnotation.String(): ""},
			map[string]string{
				"cd.service.envId":   "test123",
				"cd.service.version": "v123",
				"cd.service.name":    "testService",
			}),
		wantEventExtensions: map[string]interface{}{
			"serviceenvid":   "test123",
			"serviceversion": "v123",
			"servicename":    "testService",
		},
	}}

	for _, c := range serviceDeployedTests {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := serviceRemovedEventForObjectWithCondition(c.object.(objectWithCondition))
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				extensions := got.Extensions()
				if d := cmp.Diff(c.wantEventExtensions, extensions); d != "" {
					t.Errorf("Wrong Event Extenstions %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}
