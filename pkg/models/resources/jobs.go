/*

 Copyright 2019 The KubeSphere Authors.

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
package resources

import (
	"kubesphere.io/kubesphere/pkg/informers"
	"kubesphere.io/kubesphere/pkg/params"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type jobSearcher struct {
}

func (*jobSearcher) get(namespace, name string) (interface{}, error) {
	return informers.SharedInformerFactory().Batch().V1().Jobs().Lister().Jobs(namespace).Get(name)
}

func jobStatus(item *batchv1.Job) string {
	status := ""

	for _, condition := range item.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			status = failed
		}
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			status = complete
		}
	}
	return status
}

// Exactly Match
func (*jobSearcher) match(match map[string]string, item *batchv1.Job) bool {
	for k, v := range match {
		switch k {
		case status:
			if jobStatus(item) != v {
				return false
			}
		case name:
			if item.Name != v && item.Labels[displayName] != v {
				return false
			}
		case keyword:
			if !strings.Contains(item.Name, v) && !searchFuzzy(item.Labels, "", v) && !searchFuzzy(item.Annotations, "", v) {
				return false
			}
		default:
			if item.Labels[k] != v {
				return false
			}
		}
	}
	return true
}

func (*jobSearcher) fuzzy(fuzzy map[string]string, item *batchv1.Job) bool {

	for k, v := range fuzzy {
		switch k {
		case name:
			if !strings.Contains(item.Name, v) && !strings.Contains(item.Labels[displayName], v) {
				return false
			}
		case label:
			if !searchFuzzy(item.Labels, "", v) {
				return false
			}
		case annotation:
			if !searchFuzzy(item.Annotations, "", v) {
				return false
			}
			return false
		case app:
			if !strings.Contains(item.Labels[chart], v) && !strings.Contains(item.Labels[release], v) {
				return false
			}
		default:
			if !searchFuzzy(item.Labels, k, v) && !searchFuzzy(item.Annotations, k, v) {
				return false
			}
		}
	}

	return true
}

func jobUpdateTime(item *batchv1.Job) time.Time {
	updateTime := item.CreationTimestamp.Time
	for _, condition := range item.Status.Conditions {
		if updateTime.Before(condition.LastProbeTime.Time) {
			updateTime = condition.LastProbeTime.Time
		}
		if updateTime.Before(condition.LastTransitionTime.Time) {
			updateTime = condition.LastTransitionTime.Time
		}
	}
	return updateTime
}

func (*jobSearcher) compare(a, b *batchv1.Job, orderBy string) bool {
	switch orderBy {
	case CreateTime:
		return a.CreationTimestamp.Time.Before(b.CreationTimestamp.Time)
	case updateTime:
		return jobUpdateTime(a).Before(jobUpdateTime(b))
	case name:
		fallthrough
	default:
		return strings.Compare(a.Name, b.Name) <= 0
	}
}

func (s *jobSearcher) search(namespace string, conditions *params.Conditions, orderBy string, reverse bool) ([]interface{}, error) {
	jobs, err := informers.SharedInformerFactory().Batch().V1().Jobs().Lister().Jobs(namespace).List(labels.Everything())

	if err != nil {
		return nil, err
	}

	result := make([]*batchv1.Job, 0)

	if len(conditions.Match) == 0 && len(conditions.Fuzzy) == 0 {
		result = jobs
	} else {
		for _, item := range jobs {
			if s.match(conditions.Match, item) && s.fuzzy(conditions.Fuzzy, item) {
				result = append(result, item)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if reverse {
			tmp := i
			i = j
			j = tmp
		}
		return s.compare(result[i], result[j], orderBy)
	})

	r := make([]interface{}, 0)
	for _, i := range result {
		r = append(r, i)
	}
	return r, nil
}
