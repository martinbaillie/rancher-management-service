FROM scratch
MAINTAINER Martin Baillie <martin.t.baillie@gmail.com>

EXPOSE 8080 8081 8082

COPY ca-certificates.crt /etc/ssl/certs/
COPY bin/rancher-management-service-linux-amd64 /rancher-management-service

ENTRYPOINT ["/rancher-management-service"]
