package config

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Глобально храним одно подключение и один канал.
// Для MVP этого достаточно.
var (
	rabbitConn *amqp.Connection
	rabbitCh   *amqp.Channel
)

// RabbitMQInit:
// 1. Подключается к RabbitMQ
// 2. Открывает AMQP-канал
// 3. Создаёт exchange
// 4. Создаёт очереди
// 5. Привязывает очереди к exchange по routing key
func RabbitMQInit(cfg RabbitMQConfig) error {
	// Открываем TCP/AMQP соединение с RabbitMQ по URL из конфига.
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return fmt.Errorf("rabbitmq dial error: %w", err)
	}

	// Через соединение открываем канал.
	// В RabbitMQ почти вся работа идёт именно через channel.
	ch, err := conn.Channel()
	if err != nil {
		// Соединение уже успело открыться, поэтому его надо закрыть.
		// Ошибку от Close здесь специально не обрабатываем,
		// потому что основная причина падения уже известна — Channel() не создался.
		conn.Close()
		return fmt.Errorf("rabbitmq channel error: %w", err)
	}

	// Создаём exchange — это точка, в которую publisher будет публиковать события.
	// Тип direct значит:
	// routing key сообщения должен точно совпасть с ключом привязки очереди.
	err = ch.ExchangeDeclare(
		cfg.Exchange,     // имя exchange, например "avatars.exchange"
		cfg.ExchangeType, // тип, например "direct"
		true,             // durable: переживёт перезапуск RabbitMQ
		false,            // auto-deleted: не удалять автоматически
		false,            // internal: false, чтобы приложение могло публиковать сообщения
		false,            // no-wait: false, ждём подтверждения от сервера
		nil,              // дополнительные аргументы не нужны
	)
	if err != nil {
		// Если exchange создать не удалось, закрываем уже открытые ресурсы.
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq exchange declare error: %w", err)
	}

	// Создаём очередь для событий загрузки аватарки.
	uploadQueue, err := ch.QueueDeclare(
		cfg.QueueUpload, // имя очереди
		true,            // durable: очередь сохранится после перезапуска
		false,           // autoDelete: не удалять автоматически
		false,           // exclusive: не эксклюзивная
		false,           // noWait: ждём подтверждения создания
		nil,             // без доп. параметров
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq upload queue declare error: %w", err)
	}

	// Создаём очередь для событий удаления аватарки.
	deleteQueue, err := ch.QueueDeclare(
		cfg.QueueDelete,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq delete queue declare error: %w", err)
	}

	// Привязываем очередь загрузки к exchange.
	// Теперь все сообщения с routing key = cfg.UploadRoutingKey
	// будут попадать в очередь cfg.QueueUpload.
	err = ch.QueueBind(
		uploadQueue.Name,     // имя очереди
		cfg.UploadRoutingKey, // routing key, например "avatar.uploaded"
		cfg.Exchange,         // exchange, откуда приходят сообщения
		false,                // noWait
		nil,                  // аргументы не нужны
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq upload queue bind error: %w", err)
	}

	// Привязываем очередь удаления к exchange.
	err = ch.QueueBind(
		deleteQueue.Name,
		cfg.DeleteRoutingKey, // например "avatar.deleted"
		cfg.Exchange,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq delete queue bind error: %w", err)
	}

	// Сохраняем соединение и канал глобально,
	// чтобы потом использовать их в publisher / health-check / shutdown.
	rabbitConn = conn
	rabbitCh = ch

	return nil
}

// Возвращает текущее соединение с RabbitMQ.
func GetRabbitConn() *amqp.Connection {
	return rabbitConn
}

// Возвращает текущий канал RabbitMQ.
func GetRabbitChannel() *amqp.Channel {
	return rabbitCh
}

// Корректное закрытие ресурсов при завершении приложения.
func CloseRabbitMQ() error {
	if rabbitCh != nil {
		rabbitCh.Close()
	}
	if rabbitConn != nil {
		return rabbitConn.Close()
	}
	return nil
}
