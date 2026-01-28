package service

import "context"

type contextKey string

const operatorKey contextKey = "operator"

// OperatorInfo defines the structured identity of a user
type OperatorInfo struct {
	UserID string
	Name   string
	Role   string
}

// WithOperator injects the operator info into the context
func WithOperator(ctx context.Context, op *OperatorInfo) context.Context {
	return context.WithValue(ctx, operatorKey, op)
}

// GetOperatorInfo retrieves the operator info from the context
func GetOperatorInfo(ctx context.Context) *OperatorInfo {
	val, ok := ctx.Value(operatorKey).(*OperatorInfo)
	if !ok {
		return nil
	}
	return val
}

// GetOperator returns the username (backward compatibility)
func GetOperator(ctx context.Context) string {
	op := GetOperatorInfo(ctx)
	if op == nil {
		return "system"
	}
	return op.Name
}
