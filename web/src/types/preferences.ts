export type TerminalCursorStyle = 'block' | 'underline' | 'beam'

export interface TerminalPreferences {
  font_family: string
  font_size: number
  cursor_style: TerminalCursorStyle
  copy_on_select: boolean
  scrollback_limit: number
  enable_webgl: boolean
}

export interface SftpPreferences {
  show_hidden_files: boolean
  auto_open_queue: boolean
  confirm_before_overwrite: boolean
}

export interface SSHPreferences {
  terminal: TerminalPreferences
  sftp: SftpPreferences
}

export interface UserPreferences {
  ssh: SSHPreferences
}
