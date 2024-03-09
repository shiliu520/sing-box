# sudo add-apt-repository ppa:ozmartian/apps
# sudo apt-get update -o Acquire::http::proxy="http://127.0.0.1:7890/"
# sudo apt install golang-go -o Acquire::http::proxy="http://127.0.0.1:7890/"


go env -w CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v2
# go env -w CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOAMD64=v3
# go env -w CGO_ENABLED=0 GOOS=android GOARCH=arm64 GOARM=7
# TAGS="with_outbound_provider with_clash_api" make
go build -tags "with_outbound_provider with_clash_api with_shadowsocksr with_quic" ./cmd/sing-box

# https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip