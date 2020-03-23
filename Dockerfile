FROM golang:1.14

COPY . /app

WORKDIR /app
EXPOSE 8000

RUN go install -mod vendor go.mozilla.org/cloudops-deployment-proxy

CMD ["cloudops-deployment-proxy"]
