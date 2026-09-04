package site

import (
	"fmt"
	"log/slog"
	"strings"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/network"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type BindingStatus struct {
	connectors map[string][]string
	listeners  map[string][]string
	addresses  []network.AddressInfo
	client     internalclient.Clients
	errors     []string
	logger     *slog.Logger
}

func newBindingStatus(client internalclient.Clients, network []skupperv2alpha1.SiteRecord, addresses []network.AddressInfo) *BindingStatus {
	s := &BindingStatus{
		client:     client,
		connectors: map[string][]string{},
		listeners:  map[string][]string{},
		addresses:  addresses,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.binding_status"),
		),
	}
	s.populate(network)
	return s
}

func (s *BindingStatus) populate(network []skupperv2alpha1.SiteRecord) {
	for _, site := range network {
		for _, svc := range site.Services {
			connectors := s.connectors[svc.RoutingKey]
			for _, connector := range svc.Connectors {
				connectors = append(connectors, connector)
			}
			s.connectors[svc.RoutingKey] = connectors

			listeners := s.listeners[svc.RoutingKey]
			for _, listener := range svc.Listeners {
				listeners = append(listeners, listener)
			}
			s.listeners[svc.RoutingKey] = listeners
		}
	}
}

func (s *BindingStatus) hasMatchingConnector(routingKey string) bool {
	if len(s.connectors[routingKey]) > 0 {
		return true
	}
	for _, address := range s.addresses {
		if address.Name == routingKey {
			return address.ConnectorCount > 0
		}
	}
	return false
}

func (s *BindingStatus) hasMatchingListener(routingKey string) bool {
	if len(s.listeners[routingKey]) > 0 {
		return true
	}
	for _, address := range s.addresses {
		if address.Name == routingKey {
			return address.ListenerCount > 0
		}
	}
	return false
}

func (s *BindingStatus) updateMatchingListenerCount(connector *skupperv2alpha1.Connector) *skupperv2alpha1.Connector {
	if connector.SetHasMatchingListener(s.hasMatchingListener(connector.Spec.RoutingKey)) {
		updated, err := updateConnectorStatus(s.client, connector)
		if err != nil {
			s.logger.Error("Failed to update status for connector",
				slog.String("namespace", connector.Namespace),
				slog.String("name", connector.Name))
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) updateMatchingConnectorCount(listener *skupperv2alpha1.Listener) *skupperv2alpha1.Listener {
	if listener.SetHasMatchingConnector(s.hasMatchingConnector(listener.Spec.RoutingKey)) {
		updated, err := updateListenerStatus(s.client, listener)
		if err != nil {
			s.logger.Error("Failed to update status for listener",
				slog.String("namespace", listener.Namespace),
				slog.String("name", listener.Name))
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) updateMatchingListenerCountForAttachedConnector(connector *AttachedConnector) {
	if connector.binding != nil {
		count := len(s.listeners[connector.binding.Spec.RoutingKey])
		if count == 0 && s.hasMatchingListener(connector.binding.Spec.RoutingKey) {
			count = 1
		}
		connector.setMatchingListenerCount(count)
	}
}

func (s *BindingStatus) updateMultiKeyListenerDestination(mkl *skupperv2alpha1.MultiKeyListener) *skupperv2alpha1.MultiKeyListener {
	routingKeys := mkl.GetRoutingKeys()

	// Find which routing keys have matching connectors, preserving priority order
	var reachable []string
	for _, key := range routingKeys {
		if s.hasMatchingConnector(key) {
			reachable = append(reachable, key)
		}
	}

	hasDestination := len(reachable) > 0
	changed := mkl.SetHasDestination(hasDestination)
	if mkl.SetRoutingKeysReachable(reachable) {
		changed = true
	}

	if changed {
		updated, err := updateMultiKeyListenerStatus(s.client, mkl)
		if err != nil {
			s.logger.Error("Failed to update status for multikeylistener",
				slog.String("namespace", mkl.Namespace),
				slog.String("name", mkl.Name))
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) error() error {
	if len(s.errors) > 0 {
		return fmt.Errorf("%s", strings.Join(s.errors, ", "))
	}
	return nil
}
