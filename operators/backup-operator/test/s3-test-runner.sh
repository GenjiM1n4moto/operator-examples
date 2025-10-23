#!/bin/bash
set -e

echo "=== Building S3 MinIO Test Binary ==="

# Build test binary
cd /home/rayhe/github/operator-example/operators/backup-operator
go test -c -o /tmp/s3-minio-test ./internal/storage

echo "✅ Test binary built"
echo

# Create Kubernetes Job to run the test
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: s3-backend-test
  namespace: default
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: s3-backend-test
    spec:
      restartPolicy: Never
      containers:
      - name: test
        image: golang:1.24
        command: ["/test/s3-minio-test", "-test.v", "-test.run", "TestMinIOCompatibility"]
        volumeMounts:
        - name: test-binary
          mountPath: /test
      volumes:
      - name: test-binary
        hostPath:
          path: /tmp
          type: Directory
EOF

echo "✅ Test Job created"
echo

# Wait for pod to start
echo "Waiting for test pod to start..."
kubectl wait --for=condition=ready pod -l app=s3-backend-test -n default --timeout=30s || true

# Get pod name
POD=$(kubectl get pod -l app=s3-backend-test -n default -o jsonpath='{.items[0].metadata.name}')
echo "Test running in pod: $POD"
echo

# Follow logs
kubectl logs -f $POD -n default

# Check result
if kubectl get job s3-backend-test -n default -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' | grep -q "True"; then
  echo
  echo "🎉 Test completed successfully!"
  exit 0
else
  echo
  echo "❌ Test failed"
  exit 1
fi
