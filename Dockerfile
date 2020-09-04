FROM alpine
MAINTAINER Michael Schmidt <michael.schmidt02@@sap.com>
LABEL source_repository="https://github.com/sapcc/kube-parrot"

ADD bin/linux/parrot parrot 

ENTRYPOINT ["/parrot"]
