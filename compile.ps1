# compile
go clean -cache
go env -w CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOAMD64=v3
$env:GOPROXY="https://goproxy.io,direct"
go build -tags "with_outbound_provider with_clash_api with_shadowsocksr with_quic" ./cmd/sing-box

# launch 
# runas /env /user:administrator ".\sing-box.exe run -c .\config_win.json"

# web
# http://127.0.0.1:9090/ui/#