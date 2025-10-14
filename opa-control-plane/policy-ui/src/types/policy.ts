// Define Policy interface here if not exported from '../App'
export interface Policy {
  id?: string;
  name: string;
  path: string;
  content: string;
  active: boolean;
}