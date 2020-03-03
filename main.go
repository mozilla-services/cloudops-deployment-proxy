package main

import (
	"fmt"
	"net/http"
	"os"

	"go.mozilla.org/cloudops-deployment-proxy/proxyservice"
	"go.mozilla.org/mozlog"

	"github.com/urfave/cli"
)

func init() {
	mozlog.Logger.LoggerName = "CloudopsDeploymentProxy"
}

func main() {
	app := cli.NewApp()
	app.Name = "cloudops-deployment-proxy"
	app.Usage = "Listens for requests from dockerhub and travisci webhooks and triggers Jenkins pipelines."
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
	}

	app.Action = func(c *cli.Context) error {
		if err := validateCliContext(c); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		dockerhandler := proxyservice.NewDockerHubWebhookHandler(
			proxyservice.NewJenkins(
				c.String("jenkins-base-url"),
				c.String("jenkins-user"),
				c.String("jenkins-password"),
			),
			c.StringSlice("valid-namespace")...,
		)

		travishuandler := proxyservice.NewTravisWebhookHandler(
		// TODO Implement me to include a newjenkins service, namespace, and other things?
		// newfile proxyservice/travis.go and add handler to handlers.go and add trigger func to jenkins.go
                // add tests
		)

		mux := http.NewServeMux()
		mux.Handle("/dockerhub", dockerhandler)
		mux.Handle("/travis", githubhandler)
		mux.HandleFunc("/__heartbeat__", func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("OK"))
		})
		mux.HandleFunc("/__lbheartbeat__", func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("OK"))
		})

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

	if len(cErrors) > 0 {
		return cli.NewMultiError(cErrors...)
	}
	return nil
}
