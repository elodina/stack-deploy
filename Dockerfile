FROM ubuntu:latest
COPY stack-deploy /bin/
CMD echo $PATH
