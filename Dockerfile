FROM golang:1

COPY . /go/src/go.mozilla.org/cloudops-deployment-proxy
RUN go install go.mozilla.org/cloudops-deployment-proxy

EXPOSE 8000

CMD ["cloudops-deployment-proxy"]
