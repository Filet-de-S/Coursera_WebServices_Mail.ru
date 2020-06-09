package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"
	"net"
	"sync"
	"time"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type acl map[string][]string

type server struct {
	acl   acl
	mtx   sync.RWMutex
	chls  []chan *Event
	stats []*Stat
}

func StartMyMicroservice(ctx context.Context, listenAddr, ACLData string) error {
	var acl acl
	err := json.Unmarshal([]byte(ACLData), &acl)
	if err != nil {
		//fmt.Println("fuck can't unmarshal")
		return err
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	s := NewServer(acl)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(s.accessControlUnary),
		grpc.StreamInterceptor(s.accessControlStream),
		grpc.InTapHandle(s.inTap))

	RegisterAdminServer(server, NewAdminServer(&s))
	RegisterBizServer(server, NewBizServer())

	//fmt.Println("starting server at"+listenAddr)
	go server.Serve(lis)

	go func() {
		for {
			select {
			case <-ctx.Done():
				server.Stop()
				return
			}
		}
	}()

	return nil
}

type AdminStruct struct {
	s *server
}

func NewAdminServer(s *server) *AdminStruct {
	return &AdminStruct{s}
}

func (adm *AdminStruct) Logging(n *Nothing, stream Admin_LoggingServer) error {
	ch := make(chan *Event)
	adm.s.mtx.Lock()
	adm.s.chls = append(adm.s.chls, ch)
	adm.s.mtx.Unlock()

	for {
		select {
		case ev := <-ch:
			{
				adm.s.mtx.RLock()
				err := stream.Send(ev)
				if err != nil {
					fmt.Println("vyhozhu!", err)
					adm.s.mtx.RUnlock()
					return nil
				}
				adm.s.mtx.RUnlock()
			}
		}
	}
}

func (adm *AdminStruct) Statistics(interval *StatInterval, stream Admin_StatisticsServer) error {
	stat := &Stat{
		ByMethod:   map[string]uint64{},
		ByConsumer: map[string]uint64{},
	}
	adm.s.mtx.Lock()
	adm.s.stats = append(adm.s.stats, stat)
	adm.s.mtx.Unlock()

	timeToSend := time.NewTicker(time.Duration(interval.IntervalSeconds) * time.Second)
	defer timeToSend.Stop()

	for {
		select {
		case <-timeToSend.C:
			{
				adm.s.mtx.Lock()
				err := stream.Send(stat)
				*stat = Stat{
					ByMethod:   map[string]uint64{},
					ByConsumer: map[string]uint64{},
				}
				adm.s.mtx.Unlock()
				if err != nil {
					fmt.Println("vyhozhu! from STAT", err)
					return nil
				}
			}
		} //end select ch
	} //end for
}

type BizStruct struct {
}

func (b BizStruct) Check(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (b BizStruct) Add(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (b BizStruct) Test(ctx context.Context, nothing *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func NewBizServer() BizServer {
	return &BizStruct{}
}

func NewServer(acl acl) server {
	return server{
		acl:   acl,
		mtx:   sync.RWMutex{},
		stats: nil,
		chls:  make([]chan *Event, 0),
	}
}

func (s *server) inTap(ctx context.Context, info *tap.Info) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil
	}

	consumer, ok := md["consumer"]
	if !ok {
		return ctx, nil
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return ctx, status.Error(codes.Unauthenticated, "peer info is empty in ctx")
	}

	s.mtx.Lock()
	for i := range s.chls {
		s.chls[i] <- &Event{
			Consumer: consumer[0],
			Method:   info.FullMethodName,
			Host:     p.Addr.String(),
		}
	}

	for i := range s.stats {
		s.stats[i].ByMethod[info.FullMethodName]++
		s.stats[i].ByConsumer[consumer[0]]++
	}
	s.mtx.Unlock()

	return ctx, nil
}

func (s *server) accessControlUnary(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {

	err = s.checkACL(ctx, &info.FullMethod)
	if err != nil {
		return &Nothing{}, status.Error(codes.Unauthenticated, err.Error())
	}

	reply, err := handler(ctx, req)

	return reply, err
}

func (s *server) accessControlStream(srv interface{}, stream grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	err := s.checkACL(stream.Context(), &info.FullMethod)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	md, _ := metadata.FromIncomingContext(stream.Context())
	md.Append("method", info.FullMethod)

	//fmt.Println("meta for STREAM IS", md)

	err = handler(srv, stream)

	return err
}

func (s *server) checkACL(ctx context.Context, methodName *string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		//fmt.Println("zero metadata")
		return errors.New("zero metadata")
	}

	consumer, ok := md["consumer"]
	if !ok {
		//fmt.Println("no consumer in meta")
		return errors.New("no consumer in meta")
	}

	if accessMethods, ok := s.acl[consumer[0]]; ok {
		if !checkMethods(&accessMethods, methodName) {
			//fmt.Println("no available server method")
			return errors.New("no available server method")
		}
	} else {
		//fmt.Println("no consumer in acl")
		return errors.New("no consumer in acl")
	}
	return nil
}
func checkMethods(accessMethods *[]string, method *string) bool {
	found := false
LOOP:
	for i := range *accessMethods {
		l := len((*accessMethods)[i]) - 1

		if l > 0 && (*accessMethods)[i][l] == '*' {
			for j := 0; j < len(*method); j++ {
				if j == l {
					found = true
					break LOOP
				}

				if j > l || (*method)[j] != (*accessMethods)[i][j] {
					continue LOOP
				}
			}
		}

		if (*accessMethods)[i] == *method {
			//fmt.Println((*accessMethods)[i])
			found = true
			break
		}
	}
	return found
}
