package rpc

import (
	"context"
	"fmt"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	pb "github.com/DIMO-Network/meta-transaction-processor/pkg/grpc"
	"github.com/DIMO-Network/shared/db"
	"github.com/rs/zerolog"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MetaTransactionService struct {
	pb.MetaTransactionServiceServer
	Settings *config.Settings
	logger   *zerolog.Logger
	dbs      db.Store
}

func NewMetaTransactionService(settings *config.Settings, logger *zerolog.Logger, dbs db.Store) *MetaTransactionService {
	return &MetaTransactionService{
		Settings: settings,
		logger:   logger,
		dbs:      dbs,
	}
}

func (m *MetaTransactionService) CleanStuckMetaTransactions(ctx context.Context, in *emptypb.Empty) (*pb.CleanStuckMetaTransactionsResponse, error) {
	activeTx, err := models.MetaTransactionRequests(
		qm.OrderBy(fmt.Sprintf("%s ASC", models.MetaTransactionRequestColumns.CreatedAt)),
	).One(ctx, m.dbs.DBS().Reader)

	if err != nil {
		return nil, err
	}

	_, err = activeTx.Delete(ctx, m.dbs.DBS().Writer)
	if err != nil {
		return nil, err
	}

	return &pb.CleanStuckMetaTransactionsResponse{
		Id: activeTx.ID,
	}, nil
}
