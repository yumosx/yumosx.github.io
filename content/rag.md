---
Title: RAG 和向量数据库
Date: 2025-9-22
---

# RAG 和向量数据库

![rag](/static/images/rag.png)

## 填充

- 使用一些库对文档进行加载并提取对应的信息, 比如说 pdf, word, html
- 然后使用 text split 对文档进行分块，这部分可以使用 langchain
- 利用一个专用的模型比如 bge 将上面的分块进行向量化处理,
- 将对应的向量存储到向量库中

## 检索

- 使用专用模型对用户查询进行向量化处理
- 使用向量库对上一步的向量做相似度匹配，从而检索最相似的部分

## 生成

- 检索+ 大模型 相结合回答用户的问题



```py
embedding_model = get_embedding_model()

pdf_loader = PyPDFLoader(pdf_file, extract_images=False)
# 配置RecursiveCharacterTextSplitter分割文本块库参数，每个文本块的大小为768字符（非token），相邻文本块之间的重叠256字符（非token）
text_splitter = RecursiveCharacterTextSplitter(
    chunk_size=512, chunk_overlap=128
)

# 加载PDF文档,提取所有页的文本内容
pdf_content_list = pdf_loader.load()
# 将每页的文本内容用换行符连接，合并为PDF文档的完整文本
pdf_text = "\n".join([page.page_content for page in pdf_content_list])

# 将PDF文档文本分割成文本块Chunk
chunks = text_splitter.split_text(pdf_text)
print(f"分割的文本Chunk数量: {len(chunks)}") 

# 文本块转化为嵌入向量列表，normalize_embeddings表示对嵌入向量进行归一化，用于准确计算相似度
embeddings = []
for chunk in chunks:
    embedding = embedding_model.encode(chunk, normalize_embeddings=True)
    embeddings.append(embedding)
```


```
def retrieval_process(query, index, chunks, embedding_model, top_k=3):
    """
    检索流程：将用户查询Query转化为嵌入向量，并在Faiss索引中检索最相似的前k个文本块。
    :param query: 用户查询语句
    :param index: 已建立的Faiss向量索引
    :param chunks: 原始文本块内容列表
    :param embedding_model: 预加载的嵌入模型
    :param top_k: 返回最相似的前K个结果
    :return: 返回最相似的文本块及其相似度得分
    """
    # 将查询转化为嵌入向量，normalize_embeddings表示对嵌入向量进行归一化
    query_embedding = embedding_model.encode(query, normalize_embeddings=True)
    # 将嵌入向量转化为numpy数组，Faiss索引操作需要numpy数组输入
    query_embedding = np.array([query_embedding])

    # 在 Faiss 索引中使用 query_embedding 进行搜索，检索出最相似的前 top_k 个结果。
    # 返回查询向量与每个返回结果之间的相似度得分（在使用余弦相似度时，值越大越相似）排名列表distances，最相似的 top_k 个文本块在原始 chunks 列表中的索引indices。
    distances, indices = index.search(query_embedding, top_k)

    print(f"查询语句: {query}")
    print(f"最相似的前{top_k}个文本块:")

    # 输出查询出的top_k个文本块及其相似度得分
    results = []
    for i in range(top_k):
        # 获取相似文本块的原始内容
        result_chunk = chunks[indices[0][i]]
        print(f"文本块 {i}:\n{result_chunk}") 

        # 获取相似文本块的相似度得分
        result_distance = distances[0][i]
        print(f"相似度得分: {result_distance}\n")

        # 将相似文本块存储在结果列表中
        results.append(result_chunk)

    print("检索过程完成.")
    return results
```