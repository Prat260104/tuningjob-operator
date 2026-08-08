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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	var req SuggestRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return fmt.Errorf("failed to parse input JSON: %w", err)
	}

	resp, err := Suggest(req)
	if err != nil {
		return err
	}

	output, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal output JSON: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
