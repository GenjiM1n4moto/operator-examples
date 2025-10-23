package controller

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
)

func TestNormalizeStoredBackupsSortsByTimestamp(t *testing.T) {
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()

	backups := map[string]backupv1alpha1.StoredBackup{
		"ns/older": {
			Name:      "older",
			Namespace: "ns",
			Timestamp: &metav1.Time{Time: older},
		},
		"ns/newer": {
			Name:      "newer",
			Namespace: "ns",
			Timestamp: &metav1.Time{Time: newer},
		},
	}

	normalized := normalizeStoredBackups(backups)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(normalized))
	}

	if normalized[0].Name != "newer" {
		t.Fatalf("expected newer backup first, got %s", normalized[0].Name)
	}
}

func TestNormalizeStoredBackupsCapsHistory(t *testing.T) {
	backups := make(map[string]backupv1alpha1.StoredBackup)
	for i := 0; i < maxStatusHistory+5; i++ {
		name := fmt.Sprintf("backup-%d", i)
		backups[backupKey("ns", name)] = backupv1alpha1.StoredBackup{
			Name:      name,
			Namespace: "ns",
			Timestamp: &metav1.Time{Time: time.Now().Add(time.Duration(-i) * time.Minute)},
		}
	}

	normalized := normalizeStoredBackups(backups)
	if len(normalized) != maxStatusHistory {
		t.Fatalf("expected history to be capped at %d, got %d", maxStatusHistory, len(normalized))
	}
}
