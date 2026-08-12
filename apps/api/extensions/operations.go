package extensions

import "time"

type ContractTest struct {
	ID            string    `json:"id"`
	SchemaVersion int       `json:"schema_version"`
	Endpoint      string    `json:"endpoint"`
	Outcome       string    `json:"outcome"`
	StatusCode    int       `json:"status_code,omitempty"`
	LatencyMS     int64     `json:"latency_ms"`
	Error         string    `json:"error,omitempty"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type Notice struct {
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

type Operations struct {
	InstallationID       string              `json:"installation_id"`
	Status               string              `json:"status"`
	Requests             int                 `json:"requests"`
	SuccessfulDeliveries int                 `json:"successful_deliveries"`
	FailedDeliveries     int                 `json:"failed_deliveries"`
	DeadLetters          int                 `json:"dead_letters"`
	AverageLatencyMS     int64               `json:"average_latency_ms"`
	Invocations          int                 `json:"invocations"`
	Contributions        int                 `json:"contributions"`
	Consumption          Usage               `json:"consumption"`
	PermissionsUsed      map[string]int      `json:"permissions_used"`
	ConfigurationHistory []InstallationEvent `json:"configuration_history"`
	ContractTests        []ContractTest      `json:"contract_tests"`
	Notices              []Notice            `json:"notices"`
}

func (s *Store) Operations(repo, installation string) (Operations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Operations{}, err
	}
	for _, i := range d.Installations {
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		o := Operations{InstallationID: i.ID, Status: i.Status, Consumption: i.Usage, PermissionsUsed: map[string]int{}, ConfigurationHistory: append([]InstallationEvent(nil), i.Events...), ContractTests: append([]ContractTest(nil), i.ContractTests...)}
		var latency int64
		for _, delivery := range i.Deliveries {
			if delivery.Status == "delivered" {
				o.SuccessfulDeliveries++
			}
			if delivery.Status == "retrying" || delivery.Status == "dead_letter" {
				o.FailedDeliveries++
			}
			if delivery.Status == "dead_letter" {
				o.DeadLetters++
			}
			for _, attempt := range delivery.Attempts {
				o.Requests++
				latency += attempt.LatencyMS
			}
		}
		if o.Requests > 0 {
			o.AverageLatencyMS = latency / int64(o.Requests)
		}
		for _, action := range i.Actions {
			o.Invocations += len(action.Invocations)
		}
		o.Contributions = len(i.Contributions)
		if o.Contributions > 0 {
			o.PermissionsUsed["contributions:write"] = o.Contributions
		}
		if o.Invocations > 0 {
			o.PermissionsUsed["actions:invoke"] = o.Invocations
		}
		now := s.now().UTC()
		if i.CredentialExpiresAt == nil {
			o.Notices = append(o.Notices, Notice{Kind: "credential_missing", Severity: "warning", Message: "No active contribution credential has been issued.", Action: "rotate_credential", CreatedAt: now})
		} else if i.CredentialExpiresAt.Before(now) {
			o.Notices = append(o.Notices, Notice{Kind: "credential_expired", Severity: "critical", Message: "The installation credential has expired.", Action: "rotate_credential", CreatedAt: *i.CredentialExpiresAt})
		} else if i.CredentialExpiresAt.Before(now.Add(14 * 24 * time.Hour)) {
			o.Notices = append(o.Notices, Notice{Kind: "credential_expiring", Severity: "warning", Message: "The installation credential expires within 14 days.", Action: "rotate_credential", CreatedAt: now})
		}
		if o.DeadLetters > 0 {
			o.Notices = append(o.Notices, Notice{Kind: "broken_endpoint", Severity: "critical", Message: "Callback deliveries reached the retry limit.", Action: "test_contract_or_quarantine", CreatedAt: now})
		}
		if i.Usage.Operations >= MaxHourlyOperations*9/10 {
			o.Notices = append(o.Notices, Notice{Kind: "anomalous_consumption", Severity: "warning", Message: "Hourly operation use is above 90% of the installation limit.", Action: "narrow_or_quarantine", CreatedAt: now})
		}
		return o, nil
	}
	return Operations{}, ErrNotFound
}

func (s *Store) RecordContractTest(repo, installation, actor, endpoint, outcome, message string, code int, latency time.Duration) (ContractTest, error) {
	if actor == "" || (endpoint != "callback" && endpoint != "actions") {
		return ContractTest{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return ContractTest{}, err
	}
	for n := range d.Installations {
		i := &d.Installations[n]
		if i.ID != installation || i.RepositoryID != repo {
			continue
		}
		if i.Status == "removed" {
			return ContractTest{}, ErrForbidden
		}
		now := s.now().UTC()
		x := ContractTest{ID: id("ct"), SchemaVersion: DeliverySchemaVersion, Endpoint: endpoint, Outcome: outcome, StatusCode: code, LatencyMS: latency.Milliseconds(), Error: message, ActorID: actor, CreatedAt: now}
		i.ContractTests = append(i.ContractTests, x)
		i.Events = append(i.Events, InstallationEvent{Sequence: int64(len(i.Events) + 1), Type: "contract_tested", ActorID: actor, Reason: endpoint + ":" + outcome, CreatedAt: now})
		return x, s.save(d)
	}
	return ContractTest{}, ErrNotFound
}
