#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

TEST_TIMEOUT_SECONDS=60
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
CORE_SOURCES=(
  Sources/AnthropicProtocol.swift Sources/Config.swift Sources/Engine.swift
  Sources/GeminiPayload.swift Sources/GeminiResponse.swift
  Sources/GeminiSession.swift Sources/GeminiSessionRefresh.swift
  Sources/GeminiSessions.swift Sources/GeminiNetworking.swift
  Sources/HTTPServer.swift Sources/HTTPServer+Anthropic.swift Sources/HTTPServer+OpenAI.swift
  Sources/Models.swift Sources/Prompt.swift Sources/ToolCalling.swift Sources/Util.swift
)

run_test() {
  perl -e 'alarm shift; exec @ARGV or die "Cannot execute test: $!\n"' "$TEST_TIMEOUT_SECONDS" "$1"
}

swiftc Sources/Prompt.swift Sources/ToolCalling.swift Sources/AnthropicProtocol.swift \
  Sources/Util.swift Tests/StreamRoutingTests.swift -o "$TEST_DIR/stream-routing"
run_test "$TEST_DIR/stream-routing"

for test in ModelProtocolTests HTTPServerIntegrationTests; do
  swiftc "${CORE_SOURCES[@]}" "Tests/$test.swift" \
    -framework Network -framework CryptoKit -o "$TEST_DIR/$test"
  run_test "$TEST_DIR/$test"
done

for test in GeminiSessionTests GeminiSessionConcurrencyTests; do
  swiftc "${CORE_SOURCES[@]}" Tests/SessionTestSupport.swift "Tests/$test.swift" \
    -framework Network -framework CryptoKit -o "$TEST_DIR/$test"
  run_test "$TEST_DIR/$test"
done
