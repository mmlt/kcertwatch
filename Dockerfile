FROM scratch
MAINTAINER mmlt

USER 1001
COPY ["/bin/kcertwatch", "/"]
EXPOSE 9102
ENTRYPOINT ["/kcertwatch"]
