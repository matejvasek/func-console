// AsyncQueue is a simple FIFO queue for async producer-consumer patterns.
// `enqueue(value)` is synchronous and buffers values. `dequeue()` is async: returns
// immediately with buffered values, otherwise waits for `enqueue()`. Both throw
// if queue is closed (`enqueue` always, `dequeue` only if no buffered values).
// Supports async iteration via `for await...of` and automatic cleanup via `using`
// statements (`Symbol.dispose`).
//
// "…Stick a queue in there. Queues are the way to just get rid of this problem.
// If you're not using queues extensively, you should be.
// You should start right away, like right after this talk." -Rich Hickey
export class AsyncQueue<T> implements AsyncIterable<T>, Disposable {
  private static readonly CLOSED_ERROR = 'queue closed';

  private queue: T[] = [];
  private consumers: { resolve: (value: T) => void; reject: (e: Error) => void }[] = [];
  private closed: boolean = false;

  enqueue(value: T): void {
    if (this.closed) throw new Error(AsyncQueue.CLOSED_ERROR);
    if (this.consumers.length > 0) {
      const consumer = this.consumers.shift()!;
      consumer.resolve(value);
    } else {
      this.queue.push(value);
    }
  }

  async dequeue(): Promise<T> {
    if (this.queue.length > 0) {
      return this.queue.shift()!;
    }
    if (this.closed) throw new Error(AsyncQueue.CLOSED_ERROR);

    return new Promise<T>((resolve, reject) => {
      this.consumers.push({ resolve, reject });
    });
  }

  close() {
    if (this.closed) throw new Error(AsyncQueue.CLOSED_ERROR);
    this.closed = true;
    this.consumers.forEach((c) => {
      c.reject(new Error(AsyncQueue.CLOSED_ERROR));
    });
    this.consumers.length = 0;
  }

  [Symbol.asyncIterator]() {
    return {
      next: async (): Promise<IteratorResult<T>> => {
        try {
          const value = await this.dequeue();
          return { done: false, value };
        } catch (e) {
          if (e instanceof Error && e.message === AsyncQueue.CLOSED_ERROR) {
            return { done: true, value: undefined };
          }
          throw e;
        }
      },
    };
  }

  [Symbol.dispose](): void {
    if (!this.closed) {
      this.close();
    }
  }
}
