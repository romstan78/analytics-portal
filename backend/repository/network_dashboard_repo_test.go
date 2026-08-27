package repository

import (
	"strings"
	"testing"
)

// Фильтр по сети — контракт, на который опирается витрина: выбор сети на
// клиенте уходит сюда параметром network_id и обязан сузить область, а не
// расширить её.
func TestNetworkScopeNarrowsToSelectedNetworks(t *testing.T) {
	filter := NetworkDashboardFilter{NetworkIDs: []int{7, 12}}
	where, args := filter.networkScope("n")

	if !strings.Contains(where, "n.id IN (?,?)") {
		t.Fatalf("условие = %q, ожидалось сужение по n.id", where)
	}
	if len(args) != 2 || args[0] != 7 || args[1] != 12 {
		t.Fatalf("аргументы = %v, ожидались 7 и 12 в этом порядке", args)
	}
}

// Закрепление пользователя не может быть снято выбором сети: КАМ, выбрав
// чужую сеть, обязан получить пустую область, а не чужой портфель.
func TestNetworkScopeKeepsOwnKAMAlongsideNetworks(t *testing.T) {
	filter := NetworkDashboardFilter{OwnKAM: "Жукова Ольга", NetworkIDs: []int{3}}
	where, args := filter.networkScope("n")

	if !strings.Contains(where, "n.kam = ?") || !strings.Contains(where, "n.id IN (?)") {
		t.Fatalf("условие = %q, ожидались оба ограничения сразу", where)
	}
	if len(args) != 2 || args[0] != "Жукова Ольга" || args[1] != 3 {
		t.Fatalf("аргументы = %v, ожидались КАМ и сеть в порядке условий", args)
	}
}

// Пустой выбор означает весь доступный портфель, а не пустую витрину.
func TestNetworkScopeWithoutNetworksAddsNoClause(t *testing.T) {
	filter := NetworkDashboardFilter{}
	where, args := filter.networkScope("n")

	if where != "" || len(args) != 0 {
		t.Fatalf("условие = %q, аргументы = %v, ожидалось отсутствие ограничений", where, args)
	}
}
