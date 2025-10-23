package backup

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
)

func TestRepositoryURLForS3(t *testing.T) {
	strategy := &ExternalStrategy{}
	policy := &backupv1alpha1.BackupPolicy{}
	policy.Name = "policy"
	policy.Namespace = "control"
	policy.Spec.Destination.Type = "s3"
	policy.Spec.Destination.URL = "s3://bucket/backups"
	policy.Spec.Destination.Endpoint = "http://minio.example.svc:9000"

	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "target"}}

	repo, err := strategy.repositoryURL(policy, pvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "s3:http://minio.example.svc:9000/bucket/backups/policy/target/data"
	if repo != expected {
		t.Fatalf("expected repo %q, got %q", expected, repo)
	}
}

func TestEnsureCredentialsSecretCopiesSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := backupv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add api to scheme: %v", err)
	}

	srcSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "control"},
		Data: map[string][]byte{
			"access-key":      []byte("id"),
			"secret-key":      []byte("secret"),
			"restic-password": []byte("pw"),
		},
	}

	policy := &backupv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "control"},
		Spec: backupv1alpha1.BackupPolicySpec{
			Destination: backupv1alpha1.Destination{
				Type:              "s3",
				CredentialsSecret: "creds",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(srcSecret, policy).Build()
	strategy := &ExternalStrategy{client: fakeClient}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := strategy.ensureCredentialsSecret(ctx, "target", policy); err != nil {
		t.Fatalf("ensureCredentialsSecret returned error: %v", err)
	}

	copied := &corev1.Secret{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: "creds", Namespace: "target"}, copied); err != nil {
		t.Fatalf("copied secret not found: %v", err)
	}

	if string(copied.Data["restic-password"]) != "pw" {
		t.Fatalf("copied secret missing restic-password data")
	}

	if copied.Labels[LabelPolicy] != "policy" {
		t.Fatalf("expected policy label on copied secret, got %v", copied.Labels)
	}
	if copied.Labels[LabelManaged] != "true" {
		t.Fatalf("expected managed label on copied secret")
	}
}
