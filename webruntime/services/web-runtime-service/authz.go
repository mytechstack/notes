package main

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/rego"
)

// Embedded OPA evaluator — RFC goal G7. Policy is loaded once at startup
// and evaluated in-process on every request; no network hop to an OPA
// server.

type authorizer struct {
	query rego.PreparedEvalQuery
}

func newAuthorizer(ctx context.Context, policyPath string) (*authorizer, error) {
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("reading policy: %w", err)
	}
	q, err := rego.New(
		rego.Query("data.webruntime.authz.allow"),
		rego.Module("policy.rego", string(policy)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("preparing policy: %w", err)
	}
	return &authorizer{query: q}, nil
}

func (a *authorizer) allow(ctx context.Context, roles, required []string) (bool, error) {
	if roles == nil {
		roles = []string{}
	}
	if required == nil {
		required = []string{}
	}
	input := map[string]interface{}{
		"roles":    roles,
		"required": required,
	}
	results, err := a.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, err
	}
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, nil
	}
	allowed, _ := results[0].Expressions[0].Value.(bool)
	return allowed, nil
}
