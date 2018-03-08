package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tap "github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

func main() {
	t := tap.New()
	t.Header(0)

	os.Setenv("VALIDATION_HOOKS", "variable in parent process")

	var output string
	config := util.LifecycleConfig{
		Actions: util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete,
		PreCreate: func(r *util.Runtime) error {
			r.SetID(uuid.NewV4().String())
			g := util.GetDefaultGenerator()
			output = filepath.Join(r.BundleDir, g.Spec().Root.Path, "output")
			shPath := filepath.Join(r.BundleDir, g.Spec().Root.Path, "/bin/sh")
			err := g.AddPreStartHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo 'pre-start1 called') >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			err = g.AddPreStartHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo '$MSG') >> %s", output),
				},
				Env: []string{"MSG=pre-start2 called"},
			})
			if err != nil {
				return err
			}
			err = g.AddPostStartHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo 'post-start1 called') >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			err = g.AddPostStartHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo 'post-start2 called') >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			err = g.AddPostStopHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo 'post-stop1 called') >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			err = g.AddPostStopHook(rspec.Hook{
				Path: shPath,
				Args: []string{
					"sh", "-c", fmt.Sprintf("(echo -n $VALIDATION_HOOKS ; echo 'post-stop2 called') >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			g.SetProcessArgs([]string{"true"})
			r.SetConfig(g)
			return nil
		},
		PreDelete: func(r *util.Runtime) error {
			util.WaitingForStatus(*r, util.LifecycleStatusStopped, time.Second*10, time.Second)
			return nil
		},
	}

	err := util.RuntimeLifecycleValidate(config)
	outputData, _ := ioutil.ReadFile(output)
	expected := "pre-start1 called\npre-start2 called\npost-start1\npost-start2\npost-stop1\npost-stop2\n"
	if err == nil && string(outputData) != expected {
		err := specerror.NewError(specerror.PosixHooksCalledInOrder, fmt.Errorf("Hooks MUST be called in the listed order"), rspec.Version)
		diagnostic := map[string]string{
			"error":    err.Error(),
			"expected": expected,
			"result":   string(outputData),
			"output":   output,
		}
		t.YAML(diagnostic)
	} else {
		diagnostic := map[string]string{
			"error": err.Error(),
		}
		if e, ok := err.(*exec.ExitError); ok {
			if len(e.Stderr) > 0 {
				diagnostic["stderr"] = string(e.Stderr)
			}
		}
		t.YAML(diagnostic)
	}

	t.AutoPlan()
}
