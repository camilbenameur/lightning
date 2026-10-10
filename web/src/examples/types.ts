export type LanguageId =
  | 'python'
  | 'javascript'
  | 'typescript'
  | 'go'
  | 'rust'
  | 'c'
  | 'cpp'
  | 'java'
  | 'ruby'
  | 'php'
  | 'bash';

export interface Example {
  id: string;
  title: string;
  code: string;
  stdin?: string;
}
