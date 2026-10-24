package godaddydns

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"sync"
	"time"

	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
	extensionsv1 "github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Service struct {
	extensionsv1.UnimplementedExtensionControlServer
	dnsv1.UnimplementedDNSRecordsServer
	config     Config
	binding    Binding
	token      string
	version    string
	client     *providerClient
	mutationMu sync.Mutex
}

const reconciliationHandoffBudget = 50 * time.Millisecond

func NewService(runtime RuntimeConfig) (*Service, error) {
	if runtime.ServiceToken == "" || len(runtime.PAT) < 16 || !validVersion(runtime.Version) {
		return nil, errors.New("complete GoDaddy DNS runtime required")
	}
	return &Service{config: runtime.Config, binding: runtime.Binding, token: runtime.ServiceToken, version: runtime.Version, client: newProviderClient(runtime.Config, runtime.PAT)}, nil
}

func (s *Service) Close() { s.client.close() }

func (s *Service) authorize(ctx context.Context, correlationID, capability string) error {
	if _, ok := ctx.Deadline(); !ok {
		return status.Error(codes.InvalidArgument, "deadline required")
	}
	if !validUUID(correlationID) {
		return status.Error(codes.InvalidArgument, "valid correlation ID required")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "service identity required")
	}
	values := md.Get("x-gotth-service-token")
	if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(s.token)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid service identity")
	}
	if capability != "" && !s.binding.AllowsCapability(capability) {
		return status.Error(codes.PermissionDenied, "capability not granted")
	}
	return nil
}

func (s *Service) Handshake(ctx context.Context, in *extensionsv1.HandshakeRequest) (*extensionsv1.HandshakeResponse, error) {
	if in == nil || len(in.GetChallenge()) != 32 {
		return nil, status.Error(codes.InvalidArgument, "valid extension handshake required")
	}
	if err := s.authorize(ctx, in.GetCorrelationId(), ""); err != nil {
		return nil, err
	}
	if in.GetExpectedExtensionId() != ExtensionID || in.GetExpectedManifestSha256() != s.binding.ManifestSHA256 || in.GetExpectedGrantSha256() != s.binding.GrantSHA256 || in.GetExpectedSessionSha256() != s.binding.SessionSHA256 {
		return nil, status.Error(codes.FailedPrecondition, "extension binding mismatch")
	}
	return &extensionsv1.HandshakeResponse{Challenge: append([]byte(nil), in.GetChallenge()...), ExtensionId: ExtensionID, ExtensionVersion: s.version, ManifestSha256: s.binding.ManifestSHA256, GrantSha256: s.binding.GrantSHA256, SessionSha256: s.binding.SessionSHA256, Control: &extensionsv1.ProtocolVersion{Name: "gotth.extensions.control", Major: 1, Minor: 0}, Interfaces: []*extensionsv1.InterfaceVersion{{Name: Interface, Major: 1, Minor: 0}}, Capabilities: append([]string(nil), s.binding.Capabilities...)}, nil
}

func (s *Service) Health(ctx context.Context, in *extensionsv1.HealthRequest) (*extensionsv1.HealthResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "valid health request required")
	}
	if err := s.authorize(ctx, in.GetCorrelationId(), ""); err != nil {
		return nil, err
	}
	if in.GetSessionSha256() != s.binding.SessionSHA256 {
		return nil, status.Error(codes.FailedPrecondition, "extension session mismatch")
	}
	return &extensionsv1.HealthResponse{State: extensionsv1.HealthState_HEALTH_STATE_READY, Code: "extension.ready"}, nil
}

func (s *Service) ObserveZone(ctx context.Context, in *dnsv1.ObserveZoneRequest) (*dnsv1.ObserveZoneResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "valid observation required")
	}
	if err := s.authorize(ctx, in.GetCorrelationId(), "dns.records.observe"); err != nil {
		return nil, err
	}
	if !s.config.AllowsZone(in.GetZone()) {
		return nil, status.Error(codes.PermissionDenied, "zone not allowed")
	}
	snap, err := s.client.observe(ctx, in.GetZone(), in.GetCorrelationId())
	if err != nil {
		return nil, status.Error(codes.Unavailable, "stable provider observation unavailable")
	}
	return &dnsv1.ObserveZoneResponse{Snapshot: snapshotProto(snap)}, nil
}

func (s *Service) CreateRecord(ctx context.Context, in *dnsv1.CreateRecordRequest) (*dnsv1.MutationResponse, error) {
	if in == nil || in.GetDesired() == nil {
		return nil, status.Error(codes.InvalidArgument, "valid create required")
	}
	if err := s.authorizeMutation(ctx, in.GetCorrelationId(), in.GetOperationId(), in.GetZone(), in.GetExpectedSnapshotSha256(), "dns.records.create"); err != nil {
		return nil, err
	}
	desired, err := recordFromProto(in.GetDesired(), false)
	if err != nil || !s.config.AllowsType(desired.Type) {
		return nil, status.Error(codes.InvalidArgument, "valid allowed record required")
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	before, err := s.precondition(ctx, in.GetZone(), in.GetCorrelationId(), in.GetExpectedSnapshotSha256())
	if err != nil {
		return nil, err
	}
	if in.GetExpectedLogicalCount() > maxRecords || logicalCount(before, desired) != int(in.GetExpectedLogicalCount()) {
		return nil, status.Error(codes.FailedPrecondition, "logical record multiplicity changed")
	}
	mutationCtx, cancel, err := s.mutationContext(ctx)
	if err != nil {
		return nil, err
	}
	provider := s.client.mutate(mutationCtx, http.MethodPost, in.GetZone(), "", in.GetOperationId(), &desired)
	cancel()
	after, observeErr := s.client.observe(ctx, in.GetZone(), in.GetCorrelationId())
	return classifyCreate(before, after, desired, provider, observeErr), nil
}

func (s *Service) ReplaceRecord(ctx context.Context, in *dnsv1.ReplaceRecordRequest) (*dnsv1.MutationResponse, error) {
	if in == nil || in.GetDesired() == nil {
		return nil, status.Error(codes.InvalidArgument, "valid replacement required")
	}
	if err := s.authorizeMutation(ctx, in.GetCorrelationId(), in.GetOperationId(), in.GetZone(), in.GetExpectedSnapshotSha256(), "dns.records.replace"); err != nil {
		return nil, err
	}
	if !validOpaqueID(in.GetRecordId()) || !validHex(in.GetExpectedRecordSha256(), 64) {
		return nil, status.Error(codes.InvalidArgument, "valid record precondition required")
	}
	desired, err := recordFromProto(in.GetDesired(), false)
	if err != nil || !s.config.AllowsType(desired.Type) {
		return nil, status.Error(codes.InvalidArgument, "valid allowed record required")
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	before, err := s.precondition(ctx, in.GetZone(), in.GetCorrelationId(), in.GetExpectedSnapshotSha256())
	if err != nil {
		return nil, err
	}
	previous, ok := findRecord(before, in.GetRecordId())
	if !ok || recordDigest(previous) != in.GetExpectedRecordSha256() || !s.config.AllowsType(previous.Type) {
		return nil, status.Error(codes.FailedPrecondition, "record precondition changed")
	}
	if bytes.Equal(canonicalRecordBytes(previous, false), canonicalRecordBytes(desired, false)) {
		return nil, status.Error(codes.InvalidArgument, "replacement must change record")
	}
	mutationCtx, cancel, err := s.mutationContext(ctx)
	if err != nil {
		return nil, err
	}
	provider := s.client.mutate(mutationCtx, http.MethodPut, in.GetZone(), in.GetRecordId(), in.GetOperationId(), &desired)
	cancel()
	after, observeErr := s.client.observe(ctx, in.GetZone(), in.GetCorrelationId())
	return classifyReplace(before, after, previous, desired, provider, observeErr), nil
}

func (s *Service) DeleteRecord(ctx context.Context, in *dnsv1.DeleteRecordRequest) (*dnsv1.MutationResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "valid deletion required")
	}
	if err := s.authorizeMutation(ctx, in.GetCorrelationId(), in.GetOperationId(), in.GetZone(), in.GetExpectedSnapshotSha256(), "dns.records.delete"); err != nil {
		return nil, err
	}
	if !validOpaqueID(in.GetRecordId()) || !validHex(in.GetExpectedRecordSha256(), 64) {
		return nil, status.Error(codes.InvalidArgument, "valid record precondition required")
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	before, err := s.precondition(ctx, in.GetZone(), in.GetCorrelationId(), in.GetExpectedSnapshotSha256())
	if err != nil {
		return nil, err
	}
	previous, ok := findRecord(before, in.GetRecordId())
	if !ok || recordDigest(previous) != in.GetExpectedRecordSha256() || !s.config.AllowsType(previous.Type) {
		return nil, status.Error(codes.FailedPrecondition, "record precondition changed")
	}
	mutationCtx, cancel, err := s.mutationContext(ctx)
	if err != nil {
		return nil, err
	}
	provider := s.client.mutate(mutationCtx, http.MethodDelete, in.GetZone(), in.GetRecordId(), in.GetOperationId(), nil)
	cancel()
	after, observeErr := s.client.observe(ctx, in.GetZone(), in.GetCorrelationId())
	return classifyDelete(before, after, previous, provider, observeErr), nil
}

func (s *Service) authorizeMutation(ctx context.Context, correlationID, operationID, zone, expected, capability string) error {
	if err := s.authorize(ctx, correlationID, capability); err != nil {
		return err
	}
	if !validUUID(operationID) || operationID == correlationID || !validHex(expected, 64) {
		return status.Error(codes.InvalidArgument, "valid distinct operation precondition required")
	}
	if !s.config.AllowsZone(zone) {
		return status.Error(codes.PermissionDenied, "zone not allowed")
	}
	return nil
}

func (s *Service) mutationContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, nil, status.Error(codes.InvalidArgument, "deadline required")
	}
	remaining := time.Until(deadline)
	reserve := s.client.timeout
	if reserve <= 0 || remaining <= reserve+reconciliationHandoffBudget+10*time.Millisecond {
		return nil, nil, status.Error(codes.DeadlineExceeded, "insufficient reconciliation budget")
	}
	writeDeadline := deadline.Add(-reserve - reconciliationHandoffBudget)
	if time.Until(writeDeadline) < 10*time.Millisecond {
		return nil, nil, status.Error(codes.DeadlineExceeded, "insufficient mutation budget")
	}
	mutationCtx, cancel := context.WithDeadline(ctx, writeDeadline)
	return mutationCtx, cancel, nil
}

func (s *Service) precondition(ctx context.Context, zone, correlationID, expected string) (snapshot, error) {
	snap, err := s.client.observe(ctx, zone, correlationID)
	if err != nil {
		return snapshot{}, status.Error(codes.Unavailable, "stable provider observation unavailable")
	}
	if snap.Digest != expected {
		return snapshot{}, status.Error(codes.FailedPrecondition, "zone snapshot changed")
	}
	return snap, nil
}

func classifyCreate(before, after snapshot, desired apiRecord, provider providerResult, observeErr error) *dnsv1.MutationResponse {
	base := mutationBase(before, after, provider, observeErr)
	if observeErr != nil {
		return base
	}
	if snapshotsEqual(before, after) {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_NOT_APPLIED
		return base
	}
	newRecord, ok := exactCreateDelta(before, after, desired)
	if ok {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED
		base.Reason = dnsv1.MutationReason_MUTATION_REASON_RECONCILED
		base.AffectedRecordId = newRecord.RecordID
		base.ResultingRecord = recordProto(newRecord)
		return base
	}
	base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN
	base.Reason = dnsv1.MutationReason_MUTATION_REASON_CONCURRENT_CHANGE
	return base
}

func classifyReplace(before, after snapshot, previous, desired apiRecord, provider providerResult, observeErr error) *dnsv1.MutationResponse {
	base := mutationBase(before, after, provider, observeErr)
	base.AffectedRecordId = previous.RecordID
	base.PreviousRecord = recordProto(previous)
	if observeErr != nil {
		return base
	}
	if snapshotsEqual(before, after) {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_NOT_APPLIED
		return base
	}
	result, ok := exactReplaceDelta(before, after, previous.RecordID, desired)
	if ok {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED
		base.Reason = dnsv1.MutationReason_MUTATION_REASON_RECONCILED
		base.ResultingRecord = recordProto(result)
		return base
	}
	base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN
	base.Reason = dnsv1.MutationReason_MUTATION_REASON_CONCURRENT_CHANGE
	return base
}

func classifyDelete(before, after snapshot, previous apiRecord, provider providerResult, observeErr error) *dnsv1.MutationResponse {
	base := mutationBase(before, after, provider, observeErr)
	base.AffectedRecordId = previous.RecordID
	base.PreviousRecord = recordProto(previous)
	if observeErr != nil {
		return base
	}
	if snapshotsEqual(before, after) {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_NOT_APPLIED
		return base
	}
	if exactDeleteDelta(before, after, previous.RecordID) {
		base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED
		base.Reason = dnsv1.MutationReason_MUTATION_REASON_RECONCILED
		return base
	}
	base.Outcome = dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN
	base.Reason = dnsv1.MutationReason_MUTATION_REASON_CONCURRENT_CHANGE
	return base
}

func mutationBase(before, after snapshot, provider providerResult, observeErr error) *dnsv1.MutationResponse {
	r := &dnsv1.MutationResponse{BeforeSnapshotSha256: before.Digest, Outcome: dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN}
	if observeErr != nil {
		r.Reason = dnsv1.MutationReason_MUTATION_REASON_OBSERVATION_FAILED
		return r
	}
	r.AfterSnapshotSha256 = after.Digest
	switch provider {
	case providerRejected:
		r.Reason = dnsv1.MutationReason_MUTATION_REASON_PROVIDER_REJECTED
	case providerUnavailable:
		r.Reason = dnsv1.MutationReason_MUTATION_REASON_PROVIDER_UNAVAILABLE
	default:
		r.Reason = dnsv1.MutationReason_MUTATION_REASON_RECONCILED
	}
	return r
}

func exactCreateDelta(before, after snapshot, desired apiRecord) (apiRecord, bool) {
	if len(after.Records) != len(before.Records)+1 {
		return apiRecord{}, false
	}
	beforeByID := recordMap(before)
	var added []apiRecord
	for _, r := range after.Records {
		if old, ok := beforeByID[r.RecordID]; ok {
			if !bytes.Equal(canonicalRecordBytes(old, true), canonicalRecordBytes(r, true)) {
				return apiRecord{}, false
			}
			delete(beforeByID, r.RecordID)
		} else {
			added = append(added, r)
		}
	}
	returnIf := len(beforeByID) == 0 && len(added) == 1 && bytes.Equal(canonicalRecordBytes(added[0], false), canonicalRecordBytes(desired, false))
	if !returnIf {
		return apiRecord{}, false
	}
	return added[0], true
}

func exactReplaceDelta(before, after snapshot, id string, desired apiRecord) (apiRecord, bool) {
	if len(after.Records) != len(before.Records) {
		return apiRecord{}, false
	}
	afterByID := recordMap(after)
	for _, old := range before.Records {
		now, ok := afterByID[old.RecordID]
		if !ok {
			return apiRecord{}, false
		}
		if old.RecordID == id {
			if !bytes.Equal(canonicalRecordBytes(now, false), canonicalRecordBytes(desired, false)) {
				return apiRecord{}, false
			}
		} else if !bytes.Equal(canonicalRecordBytes(old, true), canonicalRecordBytes(now, true)) {
			return apiRecord{}, false
		}
		delete(afterByID, old.RecordID)
	}
	if len(afterByID) != 0 {
		return apiRecord{}, false
	}
	return findRecord(after, id)
}

func exactDeleteDelta(before, after snapshot, id string) bool {
	if len(after.Records)+1 != len(before.Records) {
		return false
	}
	afterByID := recordMap(after)
	for _, old := range before.Records {
		now, ok := afterByID[old.RecordID]
		if old.RecordID == id {
			if ok {
				return false
			}
			continue
		}
		if !ok || !bytes.Equal(canonicalRecordBytes(old, true), canonicalRecordBytes(now, true)) {
			return false
		}
		delete(afterByID, old.RecordID)
	}
	return len(afterByID) == 0
}

func recordMap(s snapshot) map[string]apiRecord {
	out := make(map[string]apiRecord, len(s.Records))
	for _, r := range s.Records {
		out[r.RecordID] = r
	}
	return out
}
