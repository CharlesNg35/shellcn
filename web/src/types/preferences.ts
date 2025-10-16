export type TerminalCursorStyle = 'block' | 'underline' | 'beam'

export interface TerminalPreferences {
  font_family: string
  cursor_style: TerminalCursorStyle
  copy_on_select: boolean
}

export interface SftpPreferences {
  show_hidden_files: boolean
  auto_open_queue: boolean
}

export interface SSHPreferences {
  terminal: TerminalPreferences
  sftp: SftpPreferences
}

export interface UserPreferences {
  ssh: SSHPreferences
}
