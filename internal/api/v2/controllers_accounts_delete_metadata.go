package v2

import (
	"github.com/formancehq/ledger/internal/controller/ledger"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/formancehq/go-libs/v3/api"
	"github.com/formancehq/ledger/internal/api/common"
)

func deleteAccountMetadata(w http.ResponseWriter, r *http.Request) {
	address, err := url.PathUnescape(chi.URLParam(r, "address"))
	if err != nil {
		api.BadRequestWithDetails(w, common.ErrValidation, err, err.Error())
		return
	}

	if _, err := common.LedgerFromContext(r.Context()).
		DeleteAccountMetadata(
			r.Context(),
			getCommandParameters(r, ledger.DeleteAccountMetadata{
				Address: address,
				Key:     chi.URLParam(r, "key"),
			}),
		); err != nil {
		common.HandleCommonErrors(w, r, err)
		return
	}

	api.NoContent(w)
}
