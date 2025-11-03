package eventbus

import (
	"fmt"
	"hash/fnv"

	"event-saga/internal/domain/events"
)

func GetPartition(event events.Event, numPartitions int) (int, error) {
	if numPartitions <= 0 {
		return 0, fmt.Errorf("invalid number of partitions: %d", numPartitions)
	}

	eventType := event.Type()

	if isWalletEvent(eventType) {
		userID := extractUserID(event)
		if userID == "" {
			paymentID := extractPaymentID(event)
			if paymentID != "" {
				walletPartitions := numPartitions / 2
				partition := hashKey(paymentID) % walletPartitions
				return partition * 2, nil
			}
			return 0, fmt.Errorf("cannot determine partition for wallet event: missing user_id and payment_id")
		}

		walletPartitions := numPartitions / 2
		partition := hashKey(userID) % walletPartitions
		return partition * 2, nil
	}

	if isExternalEvent(eventType) {
		paymentID := extractPaymentID(event)
		if paymentID == "" {
			return 1, fmt.Errorf("cannot determine partition for external event: missing payment_id")
		}

		externalPartitions := numPartitions / 2
		partition := hashKey(paymentID) % externalPartitions
		return (partition * 2) + 1, nil
	}

	return 0, nil
}

func isWalletEvent(eventType string) bool {
	walletEvents := []string{
		"WalletPaymentRequested",
		"FundsDebited",
		"FundsCredited",
		"FundsInsufficient",
		"WalletPaymentCompleted",
		"WalletPaymentFailed",
	}
	for _, e := range walletEvents {
		if eventType == e {
			return true
		}
	}
	return false
}

func isExternalEvent(eventType string) bool {
	externalEvents := []string{
		"ExternalPaymentRequested",
		"PaymentSentToGateway",
		"PaymentGatewayResponse",
		"ExternalPaymentCompleted",
		"ExternalPaymentFailed",
		"PaymentGatewayTimeout",
		"PaymentRetryRequested",
	}
	for _, e := range externalEvents {
		if eventType == e {
			return true
		}
	}
	return false
}

func extractUserID(event events.Event) string {
	switch data := event.Data().(type) {
	case events.WalletPaymentRequestedData:
		return data.UserID
	case events.WalletPaymentCompletedData:
		return data.UserID
	case events.WalletPaymentFailedData:
		return data.UserID
	case events.FundsDebitedData:
		return data.UserID
	case events.FundsCreditedData:
		return data.UserID
	case events.FundsInsufficientData:
		return data.UserID
	default:
		return ""
	}
}

func extractPaymentID(event events.Event) string {
	switch data := event.Data().(type) {
	case events.WalletPaymentRequestedData:
		return data.PaymentID
	case events.WalletPaymentCompletedData:
		return data.PaymentID
	case events.WalletPaymentFailedData:
		return data.PaymentID
	case events.ExternalPaymentRequestedData:
		return data.PaymentID
	case events.ExternalPaymentCompletedData:
		return data.PaymentID
	case events.ExternalPaymentFailedData:
		return data.PaymentID
	case events.PaymentSentToGatewayData:
		return data.PaymentID
	case events.PaymentGatewayResponseData:
		return data.PaymentID
	case events.PaymentGatewayTimeoutData:
		return data.PaymentID
	case events.PaymentRetryRequestedData:
		return data.PaymentID
	case events.FundsDebitedData:
		return data.PaymentID
	case events.FundsCreditedData:
		return data.PaymentID
	case events.FundsInsufficientData:
		return data.PaymentID
	default:
		return ""
	}
}
func hashKey(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32())
}
