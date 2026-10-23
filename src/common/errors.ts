export class ClusterError extends Error {
  constructor(
    readonly reason: string,
    readonly fullMessage: string,
  ) {
    super(`Deployment error. Reason: ${reason}.`);
  }
}
