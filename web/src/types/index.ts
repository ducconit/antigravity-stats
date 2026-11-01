export interface ToolCallInfo {
  name: string;
  toolSummary: string;
}

export interface StepMetric {
  step_index: number;
  source: string;
  type: string;
  status: string;
  model_name: string;
  created_at: string;
  input_tokens: number;
  cache_read_tokens: number;
  output_tokens: number;
  total_step_tokens: number;
  cache_hit_rate: number;
  tool_calls?: ToolCallInfo[];
  thinking_preview?: string;
  content_preview?: string;
  content?: string;
  thinking?: string;
}

export interface ModelUsageStat {
  model_name: string;
  step_count: number;
  total_tokens: number;
  input_tokens: number;
  cache_read_tokens: number;
  output_tokens: number;
  overall_cache_hit_rate: number;
  cost_usd: number;
  cost_vnd: number;
}

export interface SessionMetric {
  id: string;
  title: string;
  project_name: string;
  workspace: string;
  model_name: string;
  models: ModelUsageStat[];
  step_count: number;
  total_tokens: number;
  input_tokens: number;
  cache_read_tokens: number;
  output_tokens: number;
  cache_hit_rate: number;
  cost_usd: number;
  cost_vnd: number;
  created_at: string;
  last_modified: string;
  agent_name?: string;
  parent_conversation_id?: string;
  nesting_depth?: number;
  is_subagent?: boolean;
  client_tool?: string;
  steps?: StepMetric[];
}

export interface ProjectStats {
  name: string;
  workspace_path: string;
  session_count: number;
  active_session_count: number;
  total_tokens: number;
  total_input_tokens: number;
  total_cache_read: number;
  total_output: number;
  overall_cache_hit_rate: number;
  total_cost_usd: number;
  total_cost_vnd: number;
  last_modified: string;
  models: ModelUsageStat[];
  sessions: SessionMetric[];
}

export interface DailyMetric {
  date: string;
  session_count: number;
  total_tokens: number;
  input_tokens: number;
  cache_read_tokens: number;
  output_tokens: number;
  cache_hit_rate: number;
  cost_usd: number;
  cost_vnd: number;
}

export interface ClientToolStat {
  name: string;
  icon: string;
  data_dir: string;
  session_count: number;
  total_tokens: number;
  input_tokens: number;
  cache_read_tokens: number;
  output_tokens: number;
  cache_hit_rate: number;
  cost_usd: number;
  cost_vnd: number;
  active_projects: number;
  last_activity: string;
}

export interface AgentToolStat {
  name: string;
  call_count: number;
  category: string;
  server_group?: string;
}

export interface GlobalStats {
  total_sessions: number;
  active_token_sessions: number;
  total_steps: number;
  total_tokens: number;
  total_input_tokens: number;
  total_cache_read: number;
  total_output: number;
  overall_cache_hit_rate: number;
  total_cost_usd: number;
  total_cost_vnd: number;
  model_distribution: Record<string, number>;
  models?: ModelUsageStat[];
  tools?: ClientToolStat[];
  agent_tools?: AgentToolStat[];
  daily_metrics: DailyMetric[];
  recent_sessions: SessionMetric[];
  projects: ProjectStats[];
}

export interface ConfigResponse {
  status: string;
  version: string;
  auth_required: boolean;
  data_dir?: string;
}
