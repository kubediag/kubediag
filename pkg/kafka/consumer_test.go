/*
Copyright 2021 The KubeDiag Authors.

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

package kafka

import (
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func TestKafkaMessageHeadersToString(t *testing.T) {
	tests := []struct {
		headers  []kafkago.Header
		expected string
		desc     string
	}{
		{
			headers:  []kafkago.Header{},
			expected: "",
			desc:     "empty headers",
		},
		{
			headers: []kafkago.Header{
				{
					Key:   "key1",
					Value: []byte("value1"),
				},
			},
			expected: "key1:value1",
			desc:     "single header",
		},
		{
			headers: []kafkago.Header{
				{
					Key:   "key1",
					Value: []byte("value1"),
				},
				{
					Key:   "key2",
					Value: []byte("value2"),
				},
			},
			expected: "key1:value1,key2:value2",
			desc:     "multiple header",
		},
	}

	for _, test := range tests {
		str := kafkaMessageHeadersToString(test.headers)
		assert.Equal(t, test.expected, str, test.desc)
	}
}
