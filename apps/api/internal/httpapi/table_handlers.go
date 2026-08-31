package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi/apigen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type tableHTTPHandler struct {
	service  TableService
	identity identityHTTPHandler
	realtime RealtimeService
	logger   *slog.Logger
}

func (handler tableHTTPHandler) createTable(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	created, err := handler.service.Create(request.Context(), session)
	if err != nil {
		handler.writeInternalError(writer, request, "table_create_failed", err)
		return
	}
	view, err := tableView(created.Projection)
	if err != nil {
		handler.writeInternalError(writer, request, "table_projection_failed", err)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, http.StatusCreated, apigen.CreateTableResponse{InviteCode: created.InviteCode, Table: view})
}

func (handler tableHTTPHandler) previewTable(writer http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "common.error.service_unavailable", true)
		return
	}
	preview, err := handler.service.Preview(request.Context(), request.PathValue("inviteCode"))
	if errors.Is(err, table.ErrTableNotFound) {
		handler.identity.writeError(writer, request, http.StatusNotFound, "TABLE_NOT_FOUND", "table.error.not_found", false)
		return
	}
	if err != nil {
		handler.writeInternalError(writer, request, "table_preview_failed", err)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, http.StatusOK, apigen.TablePreview{
		State:            apigen.TableState(preview.State),
		Locked:           preview.Locked,
		ParticipantCount: preview.ParticipantCount,
		Capacity:         preview.Capacity,
	})
}

func (handler tableHTTPHandler) joinTable(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	projection, err := handler.service.Join(request.Context(), request.PathValue("inviteCode"), session)
	if errors.Is(err, table.ErrTableUnavailable) {
		handler.identity.writeError(writer, request, http.StatusNotFound, "TABLE_UNAVAILABLE", "table.error.unavailable", false)
		return
	}
	var domainError *table.DomainError
	if errors.As(err, &domainError) && (domainError.Code == table.ErrorTableFull || domainError.Code == table.ErrorTableLocked) {
		handler.identity.writeError(writer, request, http.StatusConflict, string(domainError.Code), tableMessageKey(domainError.Code), domainError.Code == table.ErrorTableLocked)
		return
	}
	if err != nil {
		handler.writeInternalError(writer, request, "table_join_failed", err)
		return
	}
	if handler.realtime != nil {
		handler.realtime.TableChanged(request.Context(), projection.TableID)
	}
	handler.writeProjection(writer, request, http.StatusOK, projection)
}

func (handler tableHTTPHandler) getTable(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	projection, err := handler.service.Get(request.Context(), request.PathValue("tableId"), session)
	if errors.Is(err, table.ErrTableNotFound) {
		handler.identity.writeError(writer, request, http.StatusNotFound, "TABLE_NOT_FOUND", "table.error.not_found", false)
		return
	}
	if err != nil {
		handler.writeInternalError(writer, request, "table_get_failed", err)
		return
	}
	handler.writeProjection(writer, request, http.StatusOK, projection)
}

func (handler tableHTTPHandler) leaveTable(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	err := handler.service.Leave(request.Context(), request.PathValue("tableId"), session)
	if errors.Is(err, table.ErrTableNotFound) {
		handler.identity.writeError(writer, request, http.StatusNotFound, "TABLE_NOT_FOUND", "table.error.not_found", false)
		return
	}
	var domainError *table.DomainError
	if errors.As(err, &domainError) {
		handler.identity.writeError(writer, request, http.StatusConflict, string(domainError.Code), tableMessageKey(domainError.Code), false)
		return
	}
	if err != nil {
		handler.writeInternalError(writer, request, "table_leave_failed", err)
		return
	}
	if handler.realtime != nil {
		handler.realtime.TableChanged(request.Context(), request.PathValue("tableId"))
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (handler tableHTTPHandler) authenticate(writer http.ResponseWriter, request *http.Request) (identity.Session, bool) {
	if handler.service == nil {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "common.error.service_unavailable", true)
		return identity.Session{}, false
	}
	return handler.identity.authenticate(writer, request)
}

func (handler tableHTTPHandler) writeProjection(writer http.ResponseWriter, request *http.Request, status int, projection table.Projection) {
	view, err := tableView(projection)
	if err != nil {
		handler.writeInternalError(writer, request, "table_projection_failed", err)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, status, view)
}

func (handler tableHTTPHandler) writeInternalError(writer http.ResponseWriter, request *http.Request, event string, err error) {
	handler.logger.ErrorContext(request.Context(), event,
		"request_id", requestIDFromContext(request.Context()),
		"result_code", "INTERNAL_ERROR",
		"error", err,
	)
	handler.identity.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
}

func tableView(projection table.Projection) (apigen.TableView, error) {
	tableID, err := uuid.Parse(projection.TableID)
	if err != nil {
		return apigen.TableView{}, err
	}
	viewerParticipantID, err := uuid.Parse(projection.ViewerParticipantID)
	if err != nil {
		return apigen.TableView{}, err
	}
	view := apigen.TableView{
		TableId:             tableID,
		State:               apigen.TableState(projection.State),
		Locked:              projection.Locked,
		Revision:            projection.Revision,
		LastSeq:             projection.LastSeq,
		BoardNumber:         projection.BoardNumber,
		ViewerParticipantId: viewerParticipantID,
		ViewerRole:          apigen.TableRole(projection.ViewerRole),
		Participants:        make([]apigen.TableParticipant, 0, len(projection.Participants)),
		Seats:               make(map[string]apigen.TableSeatAssignment, len(projection.Seats)),
	}
	if projection.BoardID != "" {
		boardID, parseErr := uuid.Parse(projection.BoardID)
		if parseErr != nil {
			return apigen.TableView{}, parseErr
		}
		view.BoardId = &boardID
	}
	if projection.ViewerSeat.Valid() {
		seat := apigen.TableSeat(projection.ViewerSeat)
		view.ViewerSeat = &seat
	}
	for _, participant := range projection.Participants {
		participantID, parseErr := uuid.Parse(participant.ID)
		if parseErr != nil {
			return apigen.TableView{}, parseErr
		}
		view.Participants = append(view.Participants, apigen.TableParticipant{
			Id: participantID, Nickname: participant.Nickname, Role: apigen.TableRole(participant.Role), IsBot: participant.IsBot,
		})
	}
	for seat, assignment := range projection.Seats {
		participantID, parseErr := uuid.Parse(assignment.ParticipantID)
		if parseErr != nil {
			return apigen.TableView{}, parseErr
		}
		projectedAssignment := apigen.TableSeatAssignment{
			ParticipantId: participantID, Ready: assignment.Ready, ControllerEpoch: assignment.ControllerEpoch,
		}
		if assignment.IsBot {
			isBot := true
			projectedAssignment.IsBot = &isBot
		}
		view.Seats[string(seat)] = projectedAssignment
	}
	return view, nil
}

func tableMessageKey(code table.ErrorCode) string {
	switch code {
	case table.ErrorTableFull:
		return "table.error.full"
	case table.ErrorTableLocked:
		return "table.error.locked"
	case table.ErrorOwnerCannotLeave:
		return "table.error.owner_cannot_leave"
	case table.ErrorInvalidState:
		return "table.error.invalid_state"
	default:
		return "table.error.command_rejected"
	}
}
