#!/bin/bash
set -e

echo "🚀 Setting up WG AI Gateway development environment..."

echo "📋 Prerequisites check..."
command -v docker >/dev/null 2>&1 || { echo "❌ Docker is required but not installed. Please install Docker first."; exit 1; }
command -v kind >/dev/null 2>&1 || { echo "❌ Kind is required but not installed. Please install Kind first."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl is required but not installed. Please install kubectl first."; exit 1; }

echo "✅ All prerequisites found"

echo "🔧 Creating development environment..."
make dev-setup

echo ""
echo "🎉 Development environment setup complete!"
echo ""
echo "📝 Quick start commands:"
echo "  make example           # Deploy example Gateway and HTTPRoute"
echo "  kubectl --context kind-wg-ai-gateway apply -f config/samples/gateway-service.yaml  # Create LoadBalancer service"
echo "  make logs              # View controller logs"
echo "  make dev-teardown      # Clean up when done"
echo ""
echo "🌐 Get the LoadBalancer IP for testing:"
echo "  kubectl --context kind-wg-ai-gateway get svc httpbin-gateway-service"
echo "  # Then test with: curl http://EXTERNAL-IP/get"
echo ""
echo "🔍 Useful kubectl commands:"
echo "  kubectl --context kind-wg-ai-gateway get gateways"
echo "  kubectl --context kind-wg-ai-gateway get httproutes"
echo "  kubectl --context kind-wg-ai-gateway get backends"
