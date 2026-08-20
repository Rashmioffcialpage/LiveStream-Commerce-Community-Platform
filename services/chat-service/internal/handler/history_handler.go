package handler

import "net/http"

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("id")
	messages, err := h.DB.History(r.Context(), streamID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch history")
		return
	}
	reverseChatMessages(messages)
	writeJSON(w, http.StatusOK, messages)
}
