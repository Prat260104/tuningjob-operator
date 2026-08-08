/*
Copyright 2026.

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

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

func NewProductionSuggester(binaryPath string) func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error) {
	return func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error) {
		inputJSON, err := json.Marshal(input)
		if err != nil {
			return SuggesterOutput{}, fmt.Errorf("failed to marshal suggester input: %w", err)
		}

		cmd := exec.CommandContext(ctx, binaryPath)
		cmd.Stdin = bytes.NewReader(inputJSON)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return SuggesterOutput{}, fmt.Errorf("suggester binary failed (exit %v): %s", err, stderr.String())
		}

		var output SuggesterOutput
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			return SuggesterOutput{}, fmt.Errorf("failed to parse suggester output: %w (output: %s)", err, stdout.String())
		}

		return output, nil
	}
}
