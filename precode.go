package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1, 2, 3 и т.д.
// Отправляет их в канал ch, вызывая функцию fn для каждого числа.
// Функция прекращает выполнение при получении сигнала ctx.Done().
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	defer close(ch) // Закрываем канал при завершении
	i := int64(1)   // Начинаем с 1
	for {
		select {
		case <-ctx.Done():
			return
		case ch <- i: // Отправляем число в канал
			fn(i) // Вызываем функцию для обработки числа
			i++
		}
	}
}

// Worker читает число из канала in и пишет его в канал out,
// делая паузу в 1 миллисекунду после каждой записи.
func Worker(in <-chan int64, out chan<- int64) {
	defer close(out) // Закрываем выходной канал при завершении
	for {
		v, ok := <-in // Читаем из входного канала
		if !ok {
			return // Выходим, если канал закрыт
		}
		out <- v                         // Записываем число в выходной канал
		time.Sleep(1 * time.Millisecond) // Пауза
	}
}

func main() {
	chIn := make(chan int64)

	// 3. Создание контекста
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel() // Обязательно освобождаем ресурсы

	var inputSum int64   // Сумма сгенерированных чисел
	var inputCount int64 // Количество сгенерированных чисел

	// Запускаем генератор в отдельной горутине
	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)   // Потокобезопасно увеличиваем сумму
		atomic.AddInt64(&inputCount, 1) // Потокобезопасно увеличиваем счётчик
	})

	const NumOut = 5                   // Количество рабочих горутин и каналов
	outs := make([]chan int64, NumOut) // Каналы для вывода
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i]) // Запускаем Worker для каждого канала
	}

	// 4. Собираем числа из каналов outs и передаём в chOut
	amounts := make([]int64, NumOut)  // Счётчик для каждого канала
	chOut := make(chan int64, NumOut) // Результирующий канал
	var wg sync.WaitGroup             // Синхронизация завершения горутин

	for i, out := range outs {
		wg.Add(1)
		go func(in <-chan int64, index int64) {
			defer wg.Done()
			for v := range in { // Читаем из входного канала
				chOut <- v                          // Передаём в результирующий канал
				atomic.AddInt64(&amounts[index], 1) // Увеличиваем счётчик
			}
		}(out, int64(i))
	}

	// Горутиной ждём завершения всех рабочих горутин и закрываем chOut
	go func() {
		wg.Wait()    // Ждём завершения всех Worker
		close(chOut) // Закрываем результирующий канал
	}()

	// 5. Читаем числа из результирующего канала
	var count int64 // Общее количество чисел
	var sum int64   // Общая сумма чисел

	for v := range chOut { // Читаем до закрытия канала
		atomic.AddInt64(&count, 1) // Увеличиваем счётчик
		atomic.AddInt64(&sum, v)   // Увеличиваем сумму
	}

	fmt.Println("Количество чисел:", inputCount, count)
	fmt.Println("Сумма чисел:", inputSum, sum)
	fmt.Println("Разбивка по каналам:", amounts)

	// Проверка результатов
	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}

	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: неправильная разбивка по каналам\n")
	}
}
