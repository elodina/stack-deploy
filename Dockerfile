FROM ubuntu:latest
COPY stack-deploy /bin/
COPY stacks /
CMD ls /
