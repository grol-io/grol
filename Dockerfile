FROM scratch
COPY grol /bin/grol
ENTRYPOINT ["/bin/grol"]
