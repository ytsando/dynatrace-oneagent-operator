FROM alpine:3.9

RUN apk upgrade --update --no-cache
RUN apk add ca-certificates

USER 65534:65534

ADD third_party_licenses /usr/share/dynatrace-oneagent-operator/third_party_licenses
ADD build/_output/bin/dynatrace-oneagent-operator /usr/local/bin/dynatrace-oneagent-operator
