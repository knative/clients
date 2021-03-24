// Copyright © 2019 The Knative Authors
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

package v1alpha2

import (
	"context"
	"testing"

	v1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func TestMockKnAPIServerSourceClient(t *testing.T) {

	client := NewMockKnAPIServerSourceClient(t)

	recorder := client.Recorder()

	// Record all services
	recorder.GetAPIServerSource("hello", nil, nil)
	recorder.CreateAPIServerSource(&v1alpha2.ApiServerSource{}, nil)
	recorder.UpdateAPIServerSource(&v1alpha2.ApiServerSource{}, nil)
	recorder.DeleteAPIServerSource("hello", nil)

	// Call all service
	client.GetAPIServerSource(context.Background(), "hello")
	client.CreateAPIServerSource(context.Background(), &v1alpha2.ApiServerSource{})
	client.UpdateAPIServerSource(context.Background(), &v1alpha2.ApiServerSource{})
	client.DeleteAPIServerSource(context.Background(), "hello")

	// Validate
	recorder.Validate()
}
