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

// Package main implements the stateless suggester binary for hyperparameter suggestions.
// It reads parameter specifications and past trial results from stdin as JSON, performs
// random search (ignoring history for now), and writes suggested parameter values to stdout.
// The binary is designed to be invoked once per trial and exit immediately.
package main

import (
	"fmt"
	"math/rand"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
	"github.com/Prat260104/tuningjob-operator/internal/sampling"
)

const (
	GoalMaximize = "maximize"
	GoalMinimize = "minimize"
)

type PastTrial struct {
	Parameters map[string]string `json:"parameters"`
	Metric     string            `json:"metric"`
}

type SuggestRequest struct {
	Parameters []tuningv1alpha1.ParameterSpec `json:"parameters"`
	Goal       string                         `json:"goal"`
	PastTrials []PastTrial                    `json:"pastTrials"`
}

type SuggestResponse struct {
	Parameters map[string]string `json:"parameters"`
}

// Suggest generates hyperparameter suggestions using random search.
// PastTrials is accepted but currently unused; random search doesn't need history.
// Future algorithms (e.g. Bayesian optimization) can use this same interface without
// changing the wire format.
func Suggest(req SuggestRequest) (SuggestResponse, error) {
	if req.Goal != GoalMaximize && req.Goal != GoalMinimize {
		return SuggestResponse{}, fmt.Errorf(
			"invalid goal: must be '%s' or '%s', got %s", GoalMaximize, GoalMinimize, req.Goal)
	}

	sampler := sampling.NewSampler(rand.New(rand.NewSource(rand.Int63())))

	assignments, err := sampler.SampleParameters(req.Parameters)
	if err != nil {
		return SuggestResponse{}, fmt.Errorf("failed to sample parameters: %w", err)
	}

	return SuggestResponse{Parameters: assignments}, nil
}
