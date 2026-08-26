package main

import "testing"

// Список ролей обязан совпадать с CK8_Users_role в базе: значение, прошедшее
// проверку здесь, но отвергнутое ограничением, падало бы уже на вставке.
func TestAllowedBootstrapRoles(t *testing.T) {
	for _, role := range []string{"admin", "agreement1", "agreement2", "kam"} {
		if !allowedBootstrapRole(role) {
			t.Fatalf("роль %q должна приниматься", role)
		}
	}
	for _, role := range []string{"", "Admin", "kams", "superuser"} {
		if allowedBootstrapRole(role) {
			t.Fatalf("роль %q не должна приниматься", role)
		}
	}
}

func TestUsernamePattern(t *testing.T) {
	for _, name := range []string{"demo", "kam.ershov.maksim", "kam-01", "a_b-c.d"} {
		if !usernamePattern.MatchString(name) {
			t.Fatalf("имя %q должно приниматься", name)
		}
	}
	for _, name := range []string{"ab", "Ершов", "kam ershov", "kam@example.com"} {
		if usernamePattern.MatchString(name) {
			t.Fatalf("имя %q не должно приниматься", name)
		}
	}
}
