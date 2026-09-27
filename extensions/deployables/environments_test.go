package deployables

import (
	"reflect"
	"testing"
)

func TestKeyListsByName(t *testing.T) {
	flat := []flatValue{
		{Key: "replicaCount", Value: "2"},
		{Key: "extraVars.0.name", Value: "DB_URL"},
		{Key: "extraVars.0.value", Value: "jdbc:x"},
		{Key: "extraVars.1.name", Value: "API_KEY"},
		{Key: "extraVars.1.valueFrom.secretKeyRef.name", Value: "api"},
		{Key: "ports.0", Value: "8080"},
	}
	got := keyListsByName(flat)
	want := []string{"replicaCount", "", "extraVars[DB_URL].value", "", "extraVars[API_KEY].valueFrom.secretKeyRef.name", "ports.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

// Two environments listing the same variables in another order compare by
// variable, not by position.
func TestEnvValuesComparedByVariableName(t *testing.T) {
	files := map[string]string{
		"svc/pom.xml":                `<project><artifactId>svc</artifactId><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
		"svc/src/main/java/A.java":   "class A {}",
		"svc/helm-release/dev.yaml":  "extraVars:\n  - name: A\n    value: one\n  - name: B\n    value: two\n",
		"svc/helm-release/prod.yaml": "extraVars:\n  - name: B\n    value: two\n  - name: A\n    value: one\n",
	}
	in := ws(files)
	in.RepoOf = func(string) string { return "svc" }
	m := Build(in)
	byKey := map[string][]string{}
	for _, v := range m.EnvValues {
		byKey[v.Key] = append(byKey[v.Key], v.Environment+"="+v.Value)
	}
	if len(byKey["extraVars[A].value"]) != 2 || len(byKey["extraVars[B].value"]) != 2 {
		t.Errorf("values = %v", byKey)
	}
	for k := range byKey {
		if k == "extraVars.0.value" {
			t.Errorf("positional key survived: %v", byKey)
		}
	}
}
