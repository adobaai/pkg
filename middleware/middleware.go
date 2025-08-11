package middleware

type Handler[T any] func(T) error

type Middleware[T any] func(next Handler[T]) Handler[T]

func Chain[T any](mws ...Middleware[T]) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		return func(ctx T) error {
			for i := len(mws) - 1; i >= 0; i-- {
				next = mws[i](next)
			}
			return next(ctx)
		}
	}
}
