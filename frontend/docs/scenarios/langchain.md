# LangChain

LangChain 可把提示词、模型和应用逻辑组合成工作流。下面使用 `langchain-openai` 的 `ChatOpenAI` 接入 Gkotta 的 Chat Completions，并给出可直接运行的 Python 示例。

## 安装并准备环境变量

在自己的 Python 项目环境中安装集成包：

```bash
python -m pip install langchain-openai langchain-core
```

在 [API 密钥](https://www.gkotta.bid/keys)创建密钥，确认[可用渠道](https://www.gkotta.bid/available-channels)中的模型支持 OpenAI Chat Completions，再设置变量。

macOS / Linux：

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
export GKOTTA_MODEL="YOUR_MODEL_ID"
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$env:GKOTTA_MODEL = "YOUR_MODEL_ID"
```

不要把真实密钥写进 Git、浏览器代码或 `VITE_*` 等公开环境变量。运行程序的进程需要能读取这些变量。

## 最小调用

保存为 `gkotta_chat.py`，然后在设置变量的同一终端执行 `python gkotta_chat.py`：

```python
import os
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage, SystemMessage

llm = ChatOpenAI(
    model=os.environ["GKOTTA_MODEL"],
    api_key=os.environ["GKOTTA_API_KEY"],
    base_url="https://www.gkotta.bid/v1",
    use_responses_api=False,
    timeout=60,
    max_retries=1,
)

messages = [
    SystemMessage(content="用简短中文回答。"),
    HumanMessage(content="请只回复：连接成功"),
]
reply = llm.invoke(messages)
print(reply.content)
```

`use_responses_api=False` 明确使用 Chat Completions；如果改用 Responses，需要先确认模型与分组支持该协议，参考[OpenAI API](/openai-api)。不要仅因模型名称相似就切换协议。

正常运行应输出文本。在 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对本次模型和消耗，再扩展工作流。

## 组合提示词与多轮对话

在上面的模型初始化之后，可以创建一个结构化提示词链：

```python
from langchain_core.prompts import ChatPromptTemplate
from langchain_core.output_parsers import StrOutputParser

prompt = ChatPromptTemplate.from_messages([
    ("system", "你是技术文档助手，保留专有名词，用中文解释。"),
    ("human", "请解释这个概念：{topic}"),
])
chain = prompt | llm | StrOutputParser()
print(chain.invoke({"topic": "HTTP 状态码 429"}))
```

模型本身不自动保存会话；需要多轮对话时显式保留消息：

```python
history = [HumanMessage(content="什么是 API？")]
first_reply = llm.invoke(history)
history.append(first_reply)
history.append(HumanMessage(content="给一个生活中的例子。"))
print(llm.invoke(history).content)
```

长期应用要限制保留的历史长度。不要每次把整个聊天数据库都发送给模型；输出长度和可用上下文由当前模型决定。

## 流式与异步调用

`ChatOpenAI` 同时提供流式和异步方法，无需导入不存在的 `AsyncChatOpenAI` 类。

```python
for chunk in llm.stream("用两句话解释缓存。"):
    if isinstance(chunk.content, str):
        print(chunk.content, end="", flush=True)
print()
```

```python
import asyncio

async def main():
    reply = await llm.ainvoke("用一句话解释异步请求。")
    print(reply.content)

asyncio.run(main())
```

Agent、结构化输出和 RAG 需要额外组件。Agent 使用的工具调用协议、结构化输出格式，以及 RAG 所需的嵌入接口都要单独确认支持，不要沿用聊天成功就假定其他接口也可用。

## 排错和费用核对

- **`KeyError: GKOTTA_API_KEY`**：在启动 Python 的终端或运行环境设置变量，IDE 启动的进程未必继承另一个终端的变量。
- **401/403**：检查密钥状态、有效期、IP 限制和模型分组。
- **404**：Base URL 应为 `https://www.gkotta.bid/v1`，模型 ID 和协议需匹配，路径中不应出现两次 `/v1`。
- **参数不支持**：先运行上面的最小调用，再逐项添加参数；不同模型不一定支持同样的 `temperature` 或输出限制字段。
- **429/超时**：限制并发并减少请求规模，按错误消息退避重试。SDK 重试和外层工作流重试不要无限叠加。
- **本地显示费用与账单不同**：LangChain 的内置价格表不包含 Gkotta 分组倍率及实际上游计费规则，以 Gkotta 使用记录为准。

## 官方资料

- [LangChain ChatOpenAI 参考](https://reference.langchain.com/python/integrations/langchain_openai/ChatOpenAI/)
- [官方 ChatOpenAI 实现与参数说明](https://github.com/langchain-ai/langchain/blob/master/libs/partners/openai/langchain_openai/chat_models/base.py)
- [Gkotta OpenAI API](/openai-api)
