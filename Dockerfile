FROM golang:1.19

WORKDIR /usr/share/repo
COPY . /usr/share/repo

ARG LDFLAGS=""
RUN go build \
    -ldflags "${LDFLAGS}" \
    -o containerd-registrar \
    ./cmd/registrar

FROM busybox:stable-glibc

LABEL org.opencontainers.image.author="Felix Ehrenpfort <felix@ehrenpfort.de>"
LABEL org.opencontainers.image.source="https://github.com/xinau/containerd-registrar"

COPY --from=0 /lib/x86_64-linux-gnu/libdl.so.2     /lib/libdl.so.2
COPY --from=0 /lib/x86_64-linux-gnu/libdl-2.31.so  /lib/libdl-2.31.so
COPY --from=0 /usr/share/repo/containerd-registrar /usr/bin/containerd-registrar
COPY          ./LICENSE                            /LICENSE

ENTRYPOINT [ "/usr/bin/containerd-registrar" ]
CMD [ "--help" ]
