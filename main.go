package main

import (
	"fmt"
	"net/http"
	"os"

	"go.mozilla.org/cloudops-deployment-proxy/proxyservice"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "cloudops-deployment-dockerhub-proxy"
	app.Usage = "Listens for requests from dockerhub webhooks and triggers Jenkins pipelines."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "addr, a",
			Usage: "Listen address",
			Value: "127.0.0.1:8000",
		},
	}

	app.Action = func(c *cli.Context) error {
		handler := proxyservice.NewDockerHubWebhookHandler()
		server := &http.Server{
			Addr:    c.String("addr"),
			Handler: handler,
		}
		if err := server.ListenAndServe(); err != nil {
			return cli.NewExitError(fmt.Sprintf("Server crashed: %v", err), 1)
		}
		return nil
	}
	app.Run(os.Args)
}
