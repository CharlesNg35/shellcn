export type TransferStatus = 'pending' | 'uploading' | 'completed' | 'failed'

export interface TransferItem {
  id: string
  remoteId?: string
  name: string
  path: string
  direction: string
  size: number
  uploaded: number
  status: TransferStatus
  startedAt: Date
  completedAt?: Date
  errorMessage?: string
  totalBytes?: number
  userId?: string
  userName?: string
}
