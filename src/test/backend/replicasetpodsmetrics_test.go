// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"reflect"
	"testing"

	heapster "k8s.io/heapster/api/v1/types"
)

func TestCreateMetricPath(t *testing.T) {
	cases := []struct {
		namespace  string
		podNames   []string
		metricName string
		expected   string
	}{
		{"", make([]string, 0), "", "/model/namespaces//pod-list//metrics/"},
		{"default", []string{"a", "b"}, "cpu-usage",
			"/model/namespaces/default/pod-list/a,b/metrics/cpu-usage"},
	}
	for _, c := range cases {
		actual := createMetricPath(c.namespace, c.podNames, c.metricName)
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("getReplicaSetPods(%#v, %#v, %#v) == %#v, expected %#v",
				c.namespace, c.podNames, c.metricName, actual, c.expected)
		}
	}
}

func TestUnmarshalMetrics(t *testing.T) {
	cases := []struct {
		rawData  []byte
		expected []heapster.MetricResult
	}{
		{make([]byte, 0), []heapster.MetricResult{}},
	}
	for _, c := range cases {
		actual, _ := unmarshalMetrics(c.rawData)

		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("unmarshalMetrics(%#v) == %#v, expected %#v",
				c.rawData, actual, c.expected)
		}
	}
}

func TestCreateResponse(t *testing.T) {
	cases := []struct {
		cpuMetrics []heapster.MetricResult
		memMetrics []heapster.MetricResult
		podNames   []string
		expected   *ReplicaSetMetricsByPod
	}{
		{make([]heapster.MetricResult, 0), make([]heapster.MetricResult, 0), make([]string, 0),
			&ReplicaSetMetricsByPod{
				MetricsMap: map[string]PodMetrics{},
			}},
		{[]heapster.MetricResult{
			{Metrics: []heapster.MetricPoint{
				{Value: 0},
			}},
		},
			[]heapster.MetricResult{
				{Metrics: []heapster.MetricPoint{
					{Value: 6131712},
				}},
			},
			[]string{"a", "b"},
			&ReplicaSetMetricsByPod{
				MetricsMap: map[string]PodMetrics{},
			},
		},
		{[]heapster.MetricResult{
			{Metrics: []heapster.MetricPoint{
				{Value: 0},
			}},
			{Metrics: []heapster.MetricPoint{
				{Value: 1},
			}},
		},
			[]heapster.MetricResult{
				{Metrics: []heapster.MetricPoint{
					{Value: 6131712},
				}},
				{Metrics: []heapster.MetricPoint{
					{Value: 6131712},
				}},
			},
			[]string{"a", "b"},
			&ReplicaSetMetricsByPod{
				MetricsMap: map[string]PodMetrics{
					"a": {
						CpuUsage:    0x0,
						MemoryUsage: 6131712,
					}, "b": {
						CpuUsage:    0x1,
						MemoryUsage: 6131712,
					},
				},
			},
		},
	}
	for _, c := range cases {
		actual := createResponse(c.cpuMetrics, c.memMetrics, c.podNames)

		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("createResponse(%#v, %#v, %#v) == %#v, expected %#v",
				c.cpuMetrics, c.memMetrics, c.podNames, actual, c.expected)
		}
	}
}
