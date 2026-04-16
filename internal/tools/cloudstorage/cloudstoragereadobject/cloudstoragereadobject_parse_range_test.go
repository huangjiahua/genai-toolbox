// Copyright 2026 Google LLC
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

package cloudstoragereadobject

import "testing"

func TestParseRange(t *testing.T) {
	tcs := []struct {
		in         string
		wantOffset int64
		wantLength int64
		wantErr    bool
	}{
		{in: "", wantOffset: 0, wantLength: -1},
		{in: "bytes=0-9", wantOffset: 0, wantLength: 10},
		{in: "bytes=10-19", wantOffset: 10, wantLength: 10},
		{in: "bytes=10-", wantOffset: 10, wantLength: -1},
		{in: "bytes=-5", wantOffset: -5, wantLength: -1},
		{in: "bytes=0-0", wantOffset: 0, wantLength: 1},

		{in: "garbage", wantErr: true},
		{in: "bytes=", wantErr: true},
		{in: "bytes=a-b", wantErr: true},
		{in: "bytes=-", wantErr: true},
		{in: "bytes=-0", wantErr: true},
		{in: "bytes=5-2", wantErr: true},
		{in: "bytes=-1-2", wantErr: true},
	}
	for _, tc := range tcs {
		t.Run(tc.in, func(t *testing.T) {
			offset, length, err := parseRange(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got offset=%d length=%d", offset, length)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if offset != tc.wantOffset || length != tc.wantLength {
				t.Fatalf("got (%d, %d), want (%d, %d)", offset, length, tc.wantOffset, tc.wantLength)
			}
		})
	}
}
