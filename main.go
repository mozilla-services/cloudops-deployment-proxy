package main

import (
	"fmt"
	"net/http"
	"os"

	"go.mozilla.org/cloudops-deployment-proxy/proxyservice"
	"go.mozilla.org/mozlog"

	"github.com/urfave/cli"

	"github.com/taskcluster/pulse-go/pulse"
)

func init() {
	mozlog.Logger.LoggerName = "CloudopsDeploymentProxy"
}

func main() {
	app := cli.NewApp()
	app.Name = "cloudops-deployment-dockerhub-proxy"
	app.Usage = "Listens for requests from dockerhub webhooks and triggers Jenkins pipelines."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "addr, a",
			Usage:  "Listen address",
			Value:  "127.0.0.1:8000",
			EnvVar: "ADDR",
		},
		cli.StringSliceFlag{
			Name:   "valid-namespace, n",
			Usage:  "Valid Namespace (can be used multiple times)",
			Value:  &cli.StringSlice{"mozilla"},
			EnvVar: "NAMESPACE",
		},
		cli.StringFlag{
			Name:   "jenkins-base-url",
			Usage:  "For example: https://jenkins.example",
			EnvVar: "JENKINS_BASE_URL",
		},
		cli.StringFlag{
			Name:   "jenkins-user",
			Usage:  "Username for authing against jenkins",
			EnvVar: "JENKINS_USER",
		},
		cli.StringFlag{
			Name:   "jenkins-password",
			Usage:  "Password for authing against jenkins",
			EnvVar: "JENKINS_PASSWORD",
		},
		cli.StringFlag{
			Name:   "pulse-username",
			Usage:  "Username for authing against pulse",
			EnvVar: "PULSE_USERNAME",
		},
		cli.StringFlag{
			Name:   "pulse-password",
			Usage:  "Password for authing against pulse",
			EnvVar: "PULSE_PASSWORD",
		},
		cli.StringFlag{
			Name:   "pulse-host",
			Usage:  "Pulse host to connect to",
			Value:  "amqps://pulse.mozilla.org",
			EnvVar: "PULSE_HOST",
		},
		cli.StringSliceFlag{
			Name:   "hgmo-repo",
			Usage:  "hg.mozilla.org repo to listend to pushes from (can be used multiple times)",
			Value:  &cli.StringSlice{"ci/ci-admin", "ci/ci-configuration"},
			EnvVar: "HGMO_REPO",
		},
		cli.StringFlag{
			Name:   "hgmo-pulse-queue",
			Usage:  "Pulse queue to listen to.",
			Value:  "hgmo",
			EnvVar: "HGMO_PULSE_QUEUE",
		},
		cli.StringFlag{
			Name:   "cloudops-deploy-pulse-prefix",
			Usage:  "Pulse route to listen to.",
			Value:  "cloudops.deploy.v1",
			EnvVar: "CLOUDOPS_DEPLOY_PULSE_PREFIX",
		},
		cli.StringFlag{
			Name:   "cloudops-deploy-pulse-queue",
			Usage:  "Pulse queue to listen to.",
			Value:  "deploy-proxy",
			EnvVar: "CLOUDOPS_DEPLOY_PULSE_QUEUE",
		},
		cli.StringFlag{
			Name:   "taskcluster-root-url",
			Usage:  "Taskcluster root URL",
			Value:  "https://firefox-ci-tc.services.mozilla.com",
			EnvVar: "TASKCLUSTER_ROOT_URL",
		},
	}

	app.Action = func(c *cli.Context) error {
		if err := validateCliContext(c); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		jenkins := proxyservice.NewJenkins(
			c.String("jenkins-base-url"),
			c.String("jenkins-user"),
			c.String("jenkins-password"),
		)

		dockerhubHandler := proxyservice.NewDockerHubWebhookHandler(
			jenkins,
			c.StringSlice("valid-namespace")...,
		)

		mux := http.NewServeMux()
		mux.Handle("/dockerhub", dockerhubHandler)
		mux.HandleFunc("/__heartbeat__", func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("OK"))
		})
		mux.HandleFunc("/__lbheartbeat__", func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("OK"))
		})

		if c.String("pulse-host") != "" {
			pulse := pulse.NewConnection(
				c.String("pulse-username"),
				c.String("pulse-password"),
				c.String("pulse-host"),
			)

			taskclusterPulseHandler := proxyservice.NewTaskclusterPulseHandler(
				jenkins,
				&pulse,
				c.String("cloudops-deploy-pulse-queue"),
				c.String("cloudops-deploy-pulse-prefix"),
				c.String("taskcluster-root-url"),
			)

			if err := taskclusterPulseHandler.Consume(); err != nil {
				return cli.NewExitError(fmt.Sprintf("Could not listen to taskcluster pulse: %v", err), 1)
			}

			hgmoPulseHandler := proxyservice.NewHgmoPulseHandler(
				jenkins,
				&pulse,
				c.String("hgmo-pulse-queue"),
				c.StringSlice("hgmo-repo")...,
			)

			if err := hgmoPulseHandler.Consume(); err != nil {
				return cli.NewExitError(fmt.Sprintf("Could not listen to hgmo pulse: %v", err), 1)
			}
		}

		server := &http.Server{
			Addr:    c.String("addr"),
			Handler: mux,
		}
		if err := server.ListenAndServe(); err != nil {
			return cli.NewExitError(fmt.Sprintf("Server crashed: %v", err), 1)
		}
		return nil
	}
	app.Run(os.Args)
}
func validateCliContext(c *cli.Context) error {
	cErrors := make([]error, 0)
	mustBeSet := []string{"jenkins-base-url", "jenkins-user", "jenkins-password"}
	for _, s := range mustBeSet {
		if c.String(s) == "" {
			cErrors = append(cErrors, fmt.Errorf("%s must be set", s))
		}
	}

	pulseSpecified := false
	pulseMissing := false
	pulseOptions := []string{"pulse-username", "pulse-password", "pulse-host"}
	for _, s := range pulseOptions {
		if c.String(s) == "" {
			pulseMissing = true
		} else {
			pulseSpecified = true
		}
	}
	if pulseMissing && pulseSpecified {
		cErrors = append(cErrors, fmt.Errorf("All or none of %s must be set", pulseOptions))
	}

	if len(cErrors) > 0 {
		return cli.NewMultiError(cErrors...)
	}
	return nil
}
