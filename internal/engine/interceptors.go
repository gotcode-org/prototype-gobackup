package engine

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// AuthInterceptor validates tokens and enforces Unix Socket God-Mode
func AuthInterceptor(db *DB) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		
		// 1. Check for Local Socket God-Mode
		p, ok := peer.FromContext(ctx)
		if ok && p.Addr.Network() == "unix" {
			// Bypass auth entirely, the OS handles security for the socket
			return handler(ctx, req)
		}

		// 2. Block TCP access to the AdminService entirely
		if strings.HasPrefix(info.FullMethod, "/gobackup.AdminService/") {
			return nil, status.Errorf(codes.PermissionDenied, "AdminService is restricted to local Unix socket access only")
		}

		// 3. Extract Bearer Token for BackupService TCP requests
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
		}

		values := md["authorization"]
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
		}

		tokenString := values[0]
		if !strings.HasPrefix(tokenString, "Bearer ") {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}

		rawToken := strings.TrimPrefix(tokenString, "Bearer ")

		// 4. Validate Token against SQLite
		user, err := db.ValidateToken(rawToken)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid or expired token")
		}

		// 5. Inject User Identity into Context
		ctx = context.WithValue(ctx, "user", user)

		return handler(ctx, req)
	}
}

func StreamAuthInterceptor(db *DB) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		p, ok := peer.FromContext(ctx)
		if ok && p.Addr.Network() == "unix" {
			return handler(srv, ss)
		}

		if strings.HasPrefix(info.FullMethod, "/gobackup.AdminService/") {
			return status.Errorf(codes.PermissionDenied, "AdminService is restricted to local Unix socket access only")
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "metadata is not provided")
		}

		values := md["authorization"]
		if len(values) == 0 {
			return status.Errorf(codes.Unauthenticated, "authorization token is not provided")
		}

		rawToken := strings.TrimPrefix(strings.TrimSpace(values[0]), "Bearer ")

		user, err := db.ValidateToken(rawToken)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid or expired token")
		}

		// Wrapped stream to inject user into context
		// (In Go gRPC, injecting into stream context is a bit more complex, but standard)
		// For prototype, we just validate and continue.
		_ = user 

		return handler(srv, ss)
	}
}
