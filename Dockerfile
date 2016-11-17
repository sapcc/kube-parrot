FROM alpine
MAINTAINER Michael Schmidt <michael.schmidt02@@sap.com>

ADD bin/linux/parrot parrot 

ENTRYPOINT ["/parrot"]
