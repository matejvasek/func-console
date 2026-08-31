package functions

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	"knative.dev/func/pkg/builders"
	cigithub "knative.dev/func/pkg/ci/github"
	fn "knative.dev/func/pkg/functions"

	"github.com/openshift/faas-console-plugin/backend/scm"
)

var ciGenerators = map[scm.Platform]func(string, ScaffoldConfig) error{
	scm.GitHub: generateGithubCIFiles,
}

func generateGithubCIFiles(dir string, cfg ScaffoldConfig) error {
	gen := cigithub.NewWorkflowGenerator(
		cigithub.WithWorkflowConfig(cigithub.WorkflowConfig{
			Branch:        cfg.Branch,
			RegistryLogin: !cfg.InternalRegistry,
			TestStep:      cigithub.DefaultTestStep,
		}),
		cigithub.WithWorkflowWriter(s2iBuilderWriter{inner: cigithub.DefaultWorkflowWriter}),
		cigithub.WithMessageWriter(io.Discard),
	)
	if err := gen.Generate(context.Background(), fn.Function{
		Root:    dir,
		Runtime: cfg.Runtime,
	}); err != nil {
		return fmt.Errorf("generate CI workflow: %w", err)
	}
	return nil
}

// s2iBuilderWriter wraps a WorkflowWriter and pins the generated workflow to
// the s2i builder before it is persisted. The func library computes the builder
// per runtime and hardcodes it as the FUNC_BUILDER env var on the deploy step,
// which overrides func.yaml, so we rewrite it here to keep CI consistent with
// the s2i builder default set in Generate.
type s2iBuilderWriter struct {
	inner cigithub.WorkflowWriter
}

func (w s2iBuilderWriter) Exist(path string) bool { return w.inner.Exist(path) }

func (w s2iBuilderWriter) Write(path string, raw []byte) error {
	updated, err := setWorkflowBuilder(raw, builders.S2I)
	if err != nil {
		return fmt.Errorf("set s2i builder in CI workflow: %w", err)
	}
	return w.inner.Write(path, updated)
}

// setWorkflowBuilder returns the workflow YAML with its FUNC_BUILDER env var set
// to builder, preserving the original formatting of every other node.
func setWorkflowBuilder(raw []byte, builder string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse workflow yaml: %w", err)
	}
	if !setMappingValue(&doc, "FUNC_BUILDER", builder) {
		return nil, fmt.Errorf("FUNC_BUILDER env var not found in generated workflow")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode workflow yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode workflow yaml: %w", err)
	}
	return buf.Bytes(), nil
}

// setMappingValue walks the YAML node tree and sets the scalar value of every
// mapping entry whose key equals key. It reports whether at least one entry was
// updated.
func setMappingValue(n *yaml.Node, key, value string) bool {
	found := false
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == key && v.Kind == yaml.ScalarNode {
				v.Value = value
				v.Tag = "!!str"
				v.Style = 0
				found = true
			}
		}
	}
	for _, c := range n.Content {
		if setMappingValue(c, key, value) {
			found = true
		}
	}
	return found
}
