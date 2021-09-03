package grace

type Waiter interface {
	Add(delta int)

	Done()

	Wait()
}
